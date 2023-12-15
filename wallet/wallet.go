package wallet

import (
	"context"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/lagrangedao/go-computing-provider/wallet/conf"
	"github.com/lagrangedao/go-computing-provider/wallet/contract/collateral"
	"github.com/lagrangedao/go-computing-provider/wallet/contract/swan_token"
	"github.com/lagrangedao/go-computing-provider/wallet/tablewriter"
	"golang.org/x/xerrors"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	WalletRepo  = "keystore"
	KNamePrefix = "wallet-"
)

var (
	ErrKeyInfoNotFound = fmt.Errorf("key info not found")
	ErrKeyExists       = fmt.Errorf("key already exists")
)

var reAddress = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")

func SetupWallet(dir string) (*LocalWallet, error) {
	cpPath, exit := os.LookupEnv("CP_PATH")
	if !exit {
		return nil, fmt.Errorf("missing CP_PATH env, please set export CP_PATH=xxx")
	}

	kstore, err := OpenOrInitKeystore(filepath.Join(cpPath, dir))
	if err != nil {
		return nil, err
	}

	wallet, err := NewWallet(kstore)
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

type LocalWallet struct {
	keys     map[string]*KeyInfo
	keystore KeyStore

	lk sync.Mutex
}

func NewWallet(keystore KeyStore) (*LocalWallet, error) {
	w := &LocalWallet{
		keys:     make(map[string]*KeyInfo),
		keystore: keystore,
	}
	return w, nil
}

func (w *LocalWallet) WalletSign(ctx context.Context, addr string, msg []byte) (string, error) {
	ki, err := w.findKey(addr)
	if err != nil {
		return "", err
	}
	if ki == nil {
		return "", xerrors.Errorf("signing using private key '%s': %w", addr, ErrKeyInfoNotFound)
	}
	signByte, err := Sign(ki.PrivateKey, msg)
	if err != nil {
		return "", err
	}
	return hexutil.Encode(signByte), nil
}

func (w *LocalWallet) WalletVerify(ctx context.Context, addr string, sigByte []byte, data string) (bool, error) {
	hash := crypto.Keccak256Hash([]byte(data))

	ki, err := w.findKey(addr)
	if err != nil {
		return false, err
	}
	if ki == nil {
		return false, xerrors.Errorf("signing using private key '%s': %w", addr, ErrKeyInfoNotFound)
	}

	return Verify(ki.PrivateKey, sigByte, hash.Bytes())
}

func (w *LocalWallet) findKey(addr string) (*KeyInfo, error) {
	w.lk.Lock()
	defer w.lk.Unlock()

	k, ok := w.keys[addr]
	if ok {
		return k, nil
	}
	if w.keystore == nil {
		log.Warn("findKey didn't find the key in in-memory wallet")
		return nil, nil
	}

	ki, err := w.tryFind(addr)
	if err != nil {
		if xerrors.Is(err, ErrKeyInfoNotFound) {
			return nil, nil
		}
		return nil, xerrors.Errorf("getting from keystore: %w", err)
	}

	w.keys[addr] = &ki
	return &ki, nil
}

func (w *LocalWallet) tryFind(key string) (KeyInfo, error) {
	ki, err := w.keystore.Get(KNamePrefix + key)
	if err == nil {
		return ki, err
	}

	if !xerrors.Is(err, ErrKeyInfoNotFound) {
		return KeyInfo{}, err
	}

	return ki, nil
}

func (w *LocalWallet) WalletExport(ctx context.Context, addr string) (*KeyInfo, error) {
	k, err := w.findKey(addr)
	if err != nil {
		return nil, xerrors.Errorf("failed to find key to export: %w", err)
	}
	if k == nil {
		return nil, xerrors.Errorf("private key not found for %s", addr)
	}

	return k, nil
}

func (w *LocalWallet) WalletImport(ctx context.Context, ki *KeyInfo) (string, error) {
	if ki == nil || len(strings.TrimSpace(ki.PrivateKey)) == 0 {
		return "", fmt.Errorf("not found private key")
	}

	_, publicKeyECDSA, err := ToPublic(ki.PrivateKey)
	if err != nil {
		return "", err
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()
	if err := w.keystore.Put(KNamePrefix+address, *ki); err != nil {
		return "", xerrors.Errorf("saving to keystore: %w", err)
	}
	return "", nil
}

func (w *LocalWallet) WalletList(ctx context.Context, chainName string, contractFlag bool) error {
	addressList, err := w.addressList(ctx)
	if err != nil {
		return err
	}

	addressKey := "Address"
	balanceKey := "Balance"
	nonceKey := "Nonce"
	errorKey := "Error"

	chainRpc, err := conf.GetRpcByName(chainName)
	if err != nil {
		return err
	}
	client, err := ethclient.Dial(chainRpc)
	if err != nil {
		return err
	}
	defer client.Close()

	var wallets []map[string]interface{}
	for _, addr := range addressList {
		var balance string
		if contractFlag {
			tokenStub, err := swan_token.NewTokenStub(client, swan_token.WithPublicKey(addr))
			if err == nil {
				balance, err = tokenStub.BalanceOf()
			}
		} else {
			balance, err = Balance(ctx, client, addr)
		}

		var errmsg string
		if err != nil {
			errmsg = err.Error()
		}

		nonce, err := client.PendingNonceAt(context.Background(), common.HexToAddress(addr))
		if err != nil {
			errmsg = err.Error()
		}

		wallet := map[string]interface{}{
			addressKey: addr,
			balanceKey: balance,
			errorKey:   errmsg,
			nonceKey:   nonce,
		}
		wallets = append(wallets, wallet)
	}

	tw := tablewriter.New(
		tablewriter.Col(addressKey),
		tablewriter.Col(balanceKey),
		tablewriter.Col(nonceKey),
		tablewriter.NewLineCol(errorKey))

	for _, wallet := range wallets {
		tw.Write(wallet)
	}
	return tw.Flush(os.Stdout)
}

func (w *LocalWallet) WalletNew(ctx context.Context) (string, error) {
	w.lk.Lock()
	defer w.lk.Unlock()

	privateK, err := crypto.GenerateKey()
	if err != nil {
		return "", err
	}

	privateKeyBytes := crypto.FromECDSA(privateK)
	privateKey := hexutil.Encode(privateKeyBytes)[2:]

	_, publicKeyECDSA, err := ToPublic(privateKey)
	if err != nil {
		return "", err
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA).Hex()

	keyInfo := KeyInfo{PrivateKey: privateKey}
	if err := w.keystore.Put(KNamePrefix+address, keyInfo); err != nil {
		return "", xerrors.Errorf("saving to keystore: %w", err)
	}
	w.keys[address] = &keyInfo

	return address, nil
}

func (w *LocalWallet) walletDelete(ctx context.Context, addr string) error {
	k, err := w.findKey(addr)

	if err != nil {
		return xerrors.Errorf("failed to delete key %s : %w", addr, err)
	}
	if k == nil {
		return nil // already not there
	}

	w.lk.Lock()
	defer w.lk.Unlock()

	if err := w.keystore.Delete(KNamePrefix + addr); err != nil {
		return xerrors.Errorf("failed to delete key %s: %w", addr, err)
	}

	delete(w.keys, addr)

	return nil
}

func (w *LocalWallet) WalletDelete(ctx context.Context, addr string) error {
	if err := w.walletDelete(ctx, addr); err != nil {
		return xerrors.Errorf("wallet delete: %w", err)
	}
	return nil
}

func (w *LocalWallet) WalletSend(ctx context.Context, chainName string, from, to string, amount string) (string, error) {
	chainUrl, err := conf.GetRpcByName(chainName)
	if err != nil {
		return "", err
	}
	ki, err := w.findKey(from)
	if err != nil {
		return "", err
	}
	if ki == nil {
		return "", xerrors.Errorf("the address: %s, private %w,", from, ErrKeyInfoNotFound)
	}

	client, err := ethclient.Dial(chainUrl)
	if err != nil {
		return "", err
	}
	defer client.Close()

	sendAmount, err := convertToWei(amount)
	if err != nil {
		return "", err
	}

	txHash, err := sendTransaction(client, ki.PrivateKey, to, sendAmount)
	if err != nil {
		return "", err
	}
	return txHash, nil
}

func (w *LocalWallet) WalletCollateral(ctx context.Context, chainName string, from string, amount string) (string, error) {
	sendAmount, err := convertToWei(amount)
	if err != nil {
		return "", err
	}

	chainUrl, err := conf.GetRpcByName(chainName)
	if err != nil {
		return "", err
	}
	ki, err := w.findKey(from)
	if err != nil {
		return "", err
	}
	if ki == nil {
		return "", xerrors.Errorf("the address: %s, private key %w,", from, ErrKeyInfoNotFound)
	}

	client, err := ethclient.Dial(chainUrl)
	if err != nil {
		return "", err
	}
	defer client.Close()

	tokenStub, err := swan_token.NewTokenStub(client, swan_token.WithPrivateKey(ki.PrivateKey))
	if err != nil {
		return "", err
	}

	swanTokenTxHash, err := tokenStub.Approve(sendAmount)
	if err != nil {
		return "", err
	}

	timeout := time.After(3 * time.Minute)
	ticker := time.Tick(3 * time.Second)
	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for transaction confirmation, tx: %s", swanTokenTxHash)
		case <-ticker:
			receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(swanTokenTxHash))
			if err != nil {
				if errors.Is(err, ethereum.NotFound) {
					continue
				}
				return "", fmt.Errorf("mintor swan token Approve tx, error: %+v", err)
			}

			if receipt != nil && receipt.Status == types.ReceiptStatusSuccessful {
				collateralStub, err := collateral.NewCollateralStub(client, collateral.WithPrivateKey(ki.PrivateKey))
				if err != nil {
					return "", err
				}
				collateralTxHash, err := collateralStub.Deposit(sendAmount)
				if err != nil {
					return "", err
				}
				return collateralTxHash, nil
			} else if receipt != nil && receipt.Status == 0 {
				return "", fmt.Errorf("swan token approve transaction execution failed, tx: %s", swanTokenTxHash)
			}
		}
	}
}

func (w *LocalWallet) CollateralInfo(ctx context.Context, chainName string) error {
	addrs, err := w.addressList(ctx)
	if err != nil {
		return err
	}

	addressKey := "Address"
	balanceKey := "Balance"
	collateralKey := "Collateral"
	errorKey := "Error"

	chainRpc, err := conf.GetRpcByName(chainName)
	if err != nil {
		return err
	}
	client, err := ethclient.Dial(chainRpc)
	if err != nil {
		return err
	}
	defer client.Close()

	var wallets []map[string]interface{}
	for _, addr := range addrs {
		var balance, collateralBalance string
		tokenStub, err := swan_token.NewTokenStub(client, swan_token.WithPublicKey(addr))
		if err == nil {
			balance, err = tokenStub.BalanceOf()
		}

		collateralStub, err := collateral.NewCollateralStub(client, collateral.WithPublicKey(addr))
		if err == nil {
			collateralBalance, err = collateralStub.Balances()
		}

		var errmsg string
		if err != nil {
			errmsg = err.Error()
		}

		wallet := map[string]interface{}{
			addressKey:    addr,
			balanceKey:    balance,
			collateralKey: collateralBalance,
			errorKey:      errmsg,
		}
		wallets = append(wallets, wallet)
	}

	tw := tablewriter.New(
		tablewriter.Col(addressKey),
		tablewriter.Col(balanceKey),
		tablewriter.Col(collateralKey),
		tablewriter.NewLineCol(errorKey))

	for _, wallet := range wallets {
		tw.Write(wallet)
	}
	return tw.Flush(os.Stdout)
}

func (w *LocalWallet) CollateralWithdraw(ctx context.Context, chainName string, to string, amount string) (string, error) {
	withDrawAmount, err := convertToWei(amount)
	if err != nil {
		return "", err
	}

	chainUrl, err := conf.GetRpcByName(chainName)
	if err != nil {
		return "", err
	}

	ki, err := w.findKey(to)
	if err != nil {
		return "", err
	}
	if ki == nil {
		return "", xerrors.Errorf("the address: %s, private key %w,", to, ErrKeyInfoNotFound)
	}

	client, err := ethclient.Dial(chainUrl)
	if err != nil {
		return "", err
	}
	defer client.Close()

	collateralStub, err := collateral.NewCollateralStub(client, collateral.WithPrivateKey(ki.PrivateKey))
	if err != nil {
		return "", err
	}
	withdrawHash, err := collateralStub.Withdraw(withDrawAmount)
	if err != nil {
		return "", err
	}

	return withdrawHash, nil
}

func (w *LocalWallet) addressList(ctx context.Context) ([]string, error) {
	all, err := w.keystore.List()
	if err != nil {
		return nil, xerrors.Errorf("listing keystore: %w", err)
	}

	addressList := make([]string, 0, len(all))
	for _, a := range all {
		if strings.HasPrefix(a, KNamePrefix) {
			addr := strings.TrimPrefix(a, KNamePrefix)
			addressList = append(addressList, addr)
		}
	}
	return addressList, nil
}

func convertToWei(ethValue string) (*big.Int, error) {
	ethFloat, ok := new(big.Float).SetString(ethValue)
	if !ok {
		return nil, fmt.Errorf("conversion to float failed")
	}
	weiConversion := new(big.Float).SetFloat64(1e18)
	weiFloat := new(big.Float).Mul(ethFloat, weiConversion)
	weiInt, acc := new(big.Int).SetString(weiFloat.Text('f', 0), 10)
	if !acc {
		return nil, fmt.Errorf("conversion to Wei failed")
	}
	return weiInt, nil
}
