package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/K-Kizuku/sakata-server/internal/app/service"
	"github.com/gorilla/websocket"
)

type IMessageHandler interface {
	WsHandler() func(http.ResponseWriter, *http.Request)
	ThemeHandler() func(http.ResponseWriter, *http.Request)
	JudgeHandler() func(http.ResponseWriter, *http.Request)
}

type MessageHandler struct {
	ms service.IMessageService
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var aiAnsCh = make(chan string)
var startCh = make(chan struct{}, 30)
var endCh = make(chan string)

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
		mh.ms.JoinRoom(r.Context(), conn)
		<-startCh
		go mh.ms.ReadPump(r.Context(), conn, errCh, doneCh)
		go mh.ms.WritePump(r.Context(), conn, errCh, doneCh, aiAnsCh, endCh)
		select {
		case err := <-errCh:
			log.Println("error:", err)
		case <-doneCh:
			return
		}
	}
}

func (mh *MessageHandler) ThemeHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		for range 30 {
			startCh <- struct{}{}
		}
		var body struct {
			Theme string `json:"theme"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mh.ms.PostAgent(context.TODO(), body.Theme, aiAnsCh)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func (mh *MessageHandler) JudgeHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Winner string `json:"winner"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ai, human := mh.ms.JudgeService(body.Winner)
		endCh <- body.Winner

		type result struct {
			AI    int `json:"ai"`
			Human int `json:"human"`
		}
		res := result{
			AI:    ai,
			Human: human,
		}
		if err := json.NewEncoder(w).Encode(res); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	}
}
