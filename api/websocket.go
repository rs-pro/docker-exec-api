// https://github.com/gorilla/websocket/blob/master/examples/command/main.go

package api

import (
	"encoding/json"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	dea "github.com/rs-pro/docker-exec-api"
)

type WsMessage struct {
	Kind    string          `json:"kind"`
	Command *dea.Command    `json:"command"`
	Line    *dea.OutputLine `json:"line"`
	Done    bool            `json:"done"`
}

type WsMessageJson struct {
	Kind    string          `json:"kind"`
	Command string    `json:"command"`
	Line     `json:"line"`
	Done    bool            `json:"done"`
}

func (m *WsMessage) ToJSON() []byte {
	jm := WsMessageJson{
		Kind: m.Kind
		Command: 
		Line    *dea.OutputLine `json:"line"`
		Done    bool            `json:"done"`
	}
	b, err := json.Marshal(m)
	if err != nil {
		log.Println("json marshal error", err)
		return []byte(`{"error": "failed to marshal output"}`)
	}
	return b
}

func GetMessagesToSend(ct *dea.Container, cursorCommand, cursorLine int) ([]WsMessage, int, int) {
	toSend := make([]WsMessage, 0)
	for indexCommand, command := range ct.GetCommands() {
		if cursorCommand > indexCommand {
			continue
		}
		if cursorLine == 0 || cursorCommand != indexCommand {
			cursorLine = len(command.Output)
			toSend = append(toSend, WsMessage{Kind: "command", Command: command})

			for _, line := range command.Output {
				toSend = append(toSend, WsMessage{Kind: "line", Line: line})
			}
		} else {
			for indexLine, line := range command.Output {
				if cursorLine > indexLine {
					continue
				}
				toSend = append(toSend, WsMessage{Kind: "line", Line: line})
				cursorLine = indexLine
			}
		}
		cursorCommand = indexCommand
	}

	return toSend, cursorCommand, cursorLine
}

func ConnectToContainer(c *gin.Context, ct *dea.Container) {
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		// handle error
	}
	go func() {
		defer conn.Close()

		cursorCommand, cursorLine := 0, 0

		for {
			ct.Cond.L.Lock()
			ct.Cond.Wait()

			// Get a lock on Cond to ensure messages are read out and processed correctly
			var messages []WsMessage
			messages, cursorCommand, cursorLine = GetMessagesToSend(ct, cursorCommand, cursorLine)
			ct.Cond.L.Unlock()

			// Send messages while NOT holding a lock on cond, or it will lag with multiple clients
			for _, message := range messages {
				b, err := json.Marshal(message)
				log.Println("sending message", string(b))
				if err != nil {
					log.Println("error in message json marshal", err)
					continue
				}

				err = wsutil.WriteServerMessage(conn, ws.OpText, b)
				if err != nil {
					log.Println("websocket write error", err)
					conn.Close()
					break
				}
			}
		}
	}()
}
