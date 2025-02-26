package handler

import (
	"log"
	"net/http"

	"github.com/K-Kizuku/sakata-server/internal/app/service"
	"github.com/gorilla/websocket"
)

type IMessageHandler interface {
	WsHandler() func(http.ResponseWriter, *http.Request)
}

type MessageHandler struct {
	ms service.IMessageService
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewMessageHandler(ms service.IMessageService) IMessageHandler {
	return &MessageHandler{ms: ms}
}

func (mh *MessageHandler) WsHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("error:", err)
		}
		errCh := make(chan error)
		doneCh := make(chan struct{})
		go mh.ms.ReadPump(r.Context(), conn, errCh, doneCh)
		go mh.ms.WritePump(r.Context(), conn, errCh, doneCh)
		select {
		case err := <-errCh:
			log.Println("error:", err)
		case <-doneCh:
			return
		}
	}
}
