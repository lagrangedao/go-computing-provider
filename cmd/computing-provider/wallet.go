package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/lagrangedao/go-computing-provider/wallet"
	"github.com/lagrangedao/go-computing-provider/wallet/conf"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var walletCmd = &cli.Command{
	Name:  "wallet",
	Usage: "Manage wallets",
	Subcommands: []*cli.Command{
		walletNew,
		walletList,
		walletExport,
		walletImport,
		walletDelete,
		walletSign,
		walletVerify,
		walletSend,
	},
}

var walletNew = &cli.Command{
	Name:  "new",
	Usage: "Generate a new key",
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}
		addr, err := localWallet.WalletNew(ctx)
		if err != nil {
			return err
		}
		fmt.Println(addr)

		return nil
	},
}

var walletList = &cli.Command{
	Name:  "list",
	Usage: "List wallet address",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "chain",
			Usage: "specify the account to send funds from",
			Value: conf.DefaultRpc,
		},
		&cli.BoolFlag{
			Name:  "contract",
			Usage: "specify the contract",
			Value: false,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)

		chainName := cctx.String("chain")
		if strings.TrimSpace(chainName) == "" {
			return fmt.Errorf("failed to parse chain: %s", chainName)
		}

		contractFlag := cctx.Bool("contract")

		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}

		return localWallet.WalletList(ctx, chainName, contractFlag)
	},
}

var walletExport = &cli.Command{
	Name:      "export",
	Usage:     "export keys",
	ArgsUsage: "[address]",
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}
		if !cctx.Args().Present() {
			err := fmt.Errorf("must specify key to export")
			return err
		}

		addr := cctx.Args().First()
		if err != nil {
			return err
		}

		ki, err := localWallet.WalletExport(ctx, addr)
		if err != nil {
			return err
		}

		fmt.Println(ki.PrivateKey)
		return nil
	},
}

var walletImport = &cli.Command{
	Name:      "import",
	Usage:     "import keys",
	ArgsUsage: "[<path> (optional, will read from stdin if omitted)]",
	Flags:     []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}

		var inpdata []byte
		if !cctx.Args().Present() || cctx.Args().First() == "-" {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Enter private key: ")
			indata, err := reader.ReadBytes('\n')
			if err != nil {
				return err
			}
			inpdata = indata

		} else {
			fdata, err := os.ReadFile(cctx.Args().First())
			if err != nil {
				return err
			}
			inpdata = fdata
		}

		var ki wallet.KeyInfo

		ki.PrivateKey = strings.TrimSuffix(string(inpdata), "\n")

		addr, err := localWallet.WalletImport(ctx, &ki)
		if err != nil {
			return err
		}

		fmt.Printf("imported key %s successfully!\n", addr)
		return nil
	},
}

var walletDelete = &cli.Command{
	Name:      "delete",
	Usage:     "Delete an account from the wallet",
	ArgsUsage: "<address> ",
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}

		if !cctx.Args().Present() || cctx.NArg() != 1 {
			return fmt.Errorf("must specify address to delete")
		}

		addr := cctx.Args().First()
		return localWallet.WalletDelete(ctx, addr)
	},
}

var walletSign = &cli.Command{
	Name:      "sign",
	Usage:     "Sign a message",
	ArgsUsage: "<signing address> <Message>",
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}

		if !cctx.Args().Present() || cctx.NArg() != 2 {
			return fmt.Errorf("must specify signing address and message to sign")
		}

		addr := cctx.Args().First()
		if strings.TrimSpace(addr) == "" {
			return fmt.Errorf("failed to parse sign address")
		}

		msg := cctx.Args().Get(1)
		if strings.TrimSpace(msg) == "" {
			return fmt.Errorf("failed to parse message")
		}

		sig, err := localWallet.WalletSign(ctx, addr, []byte(msg))
		if err != nil {
			return err
		}
		fmt.Println(sig)
		return nil
	},
}

var walletVerify = &cli.Command{
	Name:      "verify",
	Usage:     "verify the signature of a message",
	ArgsUsage: "<signing address>  <signature> <rawMessage>",
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)

		if cctx.NArg() != 3 {
			return fmt.Errorf("incorrect number of arguments, requires 3 parameters")
		}

		addr := cctx.Args().First()

		sigBytes, err := hexutil.Decode(cctx.Args().Get(1))
		if err != nil {
			return err
		}

		messageData := cctx.Args().Get(2)
		if strings.TrimSpace(messageData) == "" {
			return fmt.Errorf("failed to get raw message")
		}

		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}

		pass, err := localWallet.WalletVerify(ctx, addr, sigBytes, messageData)
		if err != nil {
			return err
		}
		fmt.Println(pass)
		return nil
	},
}

var walletSend = &cli.Command{
	Name:      "send",
	Usage:     "Send funds between accounts",
	ArgsUsage: "[targetAddress] [amount]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "chain",
			Usage: "specify the account to send funds from",
			Value: conf.DefaultRpc,
		},
		&cli.StringFlag{
			Name:  "from",
			Usage: "optionally specify the account to send funds from",
		},
		&cli.Uint64Flag{
			Name:  "nonce",
			Usage: "optionally specify the nonce to use",
			Value: 0,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		if cctx.NArg() != 2 {
			return fmt.Errorf(" need two params: the target address and amount")
		}

		chain := cctx.String("chain")
		if strings.TrimSpace(chain) == "" {
			return fmt.Errorf("failed to parse chain: %s", chain)
		}

		from := cctx.String("from")
		if strings.TrimSpace(from) == "" {
			return fmt.Errorf("failed to parse from address: %s", from)
		}

		to := cctx.Args().Get(0)
		if strings.TrimSpace(to) == "" {
			return fmt.Errorf("failed to parse target address: %s", to)
		}

		amount := cctx.Args().Get(1)
		if strings.TrimSpace(amount) == "" {
			return fmt.Errorf("failed to get amount: %s", chain)
		}
		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}
		txHash, err := localWallet.WalletSend(ctx, chain, from, to, amount)
		if err != nil {
			return err
		}
		fmt.Println(txHash)
		return nil
	},
}

var CollateralCmd = &cli.Command{
	Name:      "collateral",
	Usage:     "Manage the collateral amount to the hub",
	ArgsUsage: "[fromAddress] [amount]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "chain",
			Usage: "Specify which rpc connection chain to use",
			Value: conf.DefaultRpc,
		},
	},
	Subcommands: []*cli.Command{
		collateralInfoCmd,
		collateralWithdrawCmd,
	},
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		if cctx.NArg() != 2 {
			return fmt.Errorf("need two params: the from address and amount")
		}

		chain := cctx.String("chain")
		if strings.TrimSpace(chain) == "" {
			return fmt.Errorf("failed to parse chain: %s", chain)
		}

		from := cctx.Args().Get(0)
		if strings.TrimSpace(from) == "" {
			return fmt.Errorf("failed to parse from address: %s", from)
		}

		amount := cctx.Args().Get(1)
		if strings.TrimSpace(amount) == "" {
			return fmt.Errorf("failed to get amount: %s", chain)
		}

		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}
		txHash, err := localWallet.WalletCollateral(ctx, chain, from, amount)
		if err != nil {
			return err
		}
		fmt.Println(txHash)
		return nil
	},
}

var collateralInfoCmd = &cli.Command{
	Name:  "info",
	Usage: "View staking wallet details",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "chain",
			Usage: "Specify which rpc connection chain to use",
			Value: conf.DefaultRpc,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)

		chain := cctx.String("chain")
		if strings.TrimSpace(chain) == "" {
			return fmt.Errorf("failed to parse chain: %s", chain)
		}

		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}

		return localWallet.CollateralInfo(ctx, chain)
	},
}

var collateralWithdrawCmd = &cli.Command{
	Name:      "withdraw",
	Usage:     "Withdraw funds from the collateral contract",
	ArgsUsage: "[targetAddress] [amount]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "chain",
			Usage: "Specify which rpc connection chain to use",
			Value: conf.DefaultRpc,
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := reqContext(cctx)
		if cctx.NArg() != 2 {
			return fmt.Errorf("need two params: the to address and amount")
		}

		chain := cctx.String("chain")
		if strings.TrimSpace(chain) == "" {
			return fmt.Errorf("failed to parse chain: %s", chain)
		}

		to := cctx.Args().Get(0)
		if strings.TrimSpace(to) == "" {
			return fmt.Errorf("failed to parse to address: %s", to)
		}

		amount := cctx.Args().Get(1)
		if strings.TrimSpace(amount) == "" {
			return fmt.Errorf("failed to get amount: %s", chain)
		}

		localWallet, err := wallet.SetupWallet(wallet.WalletRepo)
		if err != nil {
			return err
		}
		txHash, err := localWallet.CollateralWithdraw(ctx, chain, to, amount)
		if err != nil {
			return err
		}
		fmt.Println(txHash)
		return nil
	},
}

func reqContext(cctx *cli.Context) context.Context {
	ctx, done := context.WithCancel(cctx.Context)
	sigChan := make(chan os.Signal, 2)
	go func() {
		<-sigChan
		done()
	}()
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	return ctx
}
