package computing

import (
	"bufio"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	PingMsg = "ping"
)

var upgrade = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WsClient struct {
	client           *websocket.Conn
	message          chan wsMessage
	stopCh           chan struct{}
	checkFailedCount int
}

type wsMessage struct {
	data    []byte
	msgType int
}

func NewWsClient(client *websocket.Conn) *WsClient {
	wsClient := &WsClient{
		client:           client,
		message:          make(chan wsMessage, 5),
		stopCh:           make(chan struct{}),
		checkFailedCount: 0,
	}

	client.SetCloseHandler(func(code int, text string) error {
		log.Println(code, "user client send close event")
		wsClient.Close()
		return nil
	})

	return wsClient
}

func (ws *WsClient) Close() {
	defer func() {
		if ws.client != nil {
			ws.client.Close()
		}
	}()
	close(ws.stopCh)
}

func (ws *WsClient) HandleBuildLog(logFilePath string) {
	ws.ReadMessage()
	ws.writeMessage()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ws.message <- wsMessage{
					data:    []byte(PingMsg),
					msgType: websocket.TextMessage,
				}
			case <-ws.stopCh:
				return
			}
		}
	}()

	buildLog, err := os.Open(logFilePath)
	if err != nil {
		log.Println("Failed to open build log file,", err)
		return
	}
	defer buildLog.Close()

	scanner := bufio.NewScanner(buildLog)
	for scanner.Scan() {
		select {
		case <-ws.stopCh:
			return
		default:
			ws.message <- wsMessage{
				data:    scanner.Bytes(),
				msgType: websocket.TextMessage,
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (ws *WsClient) writeMessage() {
	go func() {
		for {
			select {
			case msg := <-ws.message:
				if err := ws.client.WriteMessage(msg.msgType, msg.data); err != nil {
					log.Println("WriteMessage: ", err)
					return
				}
				if string(msg.data) == PingMsg {
					ws.client.SetReadDeadline(time.Now().Add(2 * time.Second))
				}
			case <-ws.stopCh:
				return
			}
		}
	}()
}

func (ws *WsClient) ReadMessage() {
	go func() {
		for {
			if _, _, err := ws.client.ReadMessage(); err != nil {
				if ws.checkFailedCount > 30 {
					ws.Close()
					break
				}
				ws.checkFailedCount++
				time.Sleep(600 * time.Millisecond)
			} else {
				ws.checkFailedCount = 0
			}
		}
	}()
}
