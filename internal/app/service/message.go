package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/K-Kizuku/sakata-server/internal/app/repository"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type IMessageService interface {
	ReadPump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{})
	WritePump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{}, aiAnsCh chan string, endCh chan string)
	JoinRoom(ctx context.Context, conn *websocket.Conn) error
	LeaveRoom(ctx context.Context, conn *websocket.Conn) error
	PostAgent(ctx context.Context, q string, aiAnsCh chan string)
	JudgeService(winner string) (int, int)
}

type MessageService struct {
	mr        repository.IMessageRepository
	client    []*websocket.Conn
	mux       sync.RWMutex
	timeLimit int
	AI        int
	Human     int
}

func NewMessageService(mr repository.IMessageRepository) IMessageService {
	return &MessageService{
		mr:        mr,
		client:    make([]*websocket.Conn, 0, 30),
		mux:       sync.RWMutex{},
		timeLimit: 60,
	}
}

func (ms *MessageService) JoinRoom(ctx context.Context, conn *websocket.Conn) error {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	ms.client = append(ms.client, conn)
	return nil
}

func (ms *MessageService) LeaveRoom(ctx context.Context, conn *websocket.Conn) error {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	for i, c := range ms.client {
		if c == conn {
			ms.client = append(ms.client[:i], ms.client[i+1:]...)
			break
		}
	}
	return nil
}

func (ms *MessageService) ReadPump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{}) {
	defer conn.Close()
	for {
		select {
		case <-ctx.Done():
			doneCh <- struct{}{}
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read message")
				errCh <- err
				return
			}
			var s Schema
			if err := json.Unmarshal(message, &s); err != nil {
				log.Println("json unmarshal", err)
			}

			timestamp := time.Now().Unix()
			if err := ms.mr.AddMessage(ctx, timestamp, string(message)); err != nil {
				log.Println("add message", err)

			}
			p := Schema{
				EventType: "message",
				Content:   s.Content,
				Name:      s.Name,
			}
			m, err := json.Marshal(p)
			if err != nil {
				log.Println("json marshal", err)
			}
			ms.broadcastMessage(ctx, m)
		}
	}
}

func (ms *MessageService) WritePump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{}, aiAnsCh chan string, endCh chan string) {
	defer conn.Close()
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			doneCh <- struct{}{}
			return
		case <-ticker.C:
			ms.timeLimit--
			if ms.timeLimit <= 0 {
				ms.timeLimit = 0
			}
			s := Schema{
				EventType: "timer",
				Content:   strconv.Itoa(ms.timeLimit),
			}
			m, err := json.Marshal(s)
			if err != nil {
				log.Println("json marshal", err)
			}
			ms.broadcastMessage(ctx, m)

		case ans := <-aiAnsCh:
			s := Schema{
				EventType: "answer",
				Content:   ans,
				Name:      "AI",
			}
			m, err := json.Marshal(s)
			if err != nil {
				log.Println("json marshal", err)
			}
			ms.broadcastMessage(ctx, m)
		case result := <-endCh:
			s := Schema{
				EventType: "winner",
				Content:   result,
			}
			m, err := json.Marshal(s)
			if err != nil {
				log.Println("json marshal", err)
			}
			ms.broadcastMessage(ctx, m)
			s = Schema{
				EventType: "result",
				Content:   strconv.Itoa(ms.AI) + " - " + strconv.Itoa(ms.Human),
			}
			m, err = json.Marshal(s)
			if err != nil {
				log.Println("json marshal", err)
			}
			ms.broadcastMessage(ctx, m)
			ms.AI = 0
			ms.Human = 0
		}
	}

}

type Req struct {
	Question  string `json:"question"`
	SessionID string `json:"session_id"`
}

// lambdaにリクエストを送る
func (ms *MessageService) PostAgent(ctx context.Context, q string, aiAnsCh chan string) {
	// リクエストの作成
	uuid := uuid.Must(uuid.NewV7())
	r := Req{
		Question:  q,
		SessionID: uuid.String(),
	}
	b, err := json.Marshal(r)
	if err != nil {
		log.Println("json marshal error:", err)
		return
	}
	req, err := http.NewRequest("POST", "https://6hksbfrud3xvkxoqdftd6czeee0rklcg.lambda-url.ap-northeast-1.on.aws/", bytes.NewBuffer(b))
	if err != nil {
		log.Println("リクエストの作成エラー:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// リクエストの送信
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("リクエストの送信エラー:", err)
		return
	}
	defer resp.Body.Close()

	// レスポンスの読み込み
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("レスポンスの読み込みエラー:", err)
		return
	}

	aiAnsCh <- string(body)
}

// func (ms *MessageService) getClientByRoomID(id string) iter.Seq[*websocket.Conn] {
// 	ms.mux.Lock()
// 	defer ms.mux.Unlock()
// 	return func(yield func(*websocket.Conn) bool) {
// 		for _, v := range ms.room[id] {
// 			for !yield(v) {
// 				return
// 			}
// 		}
// 	}
// }

func (ms *MessageService) JudgeService(winner string) (ai int, human int) {
	if winner == "AI" {
		ms.AI++
	} else {
		ms.Human++
	}
	return ms.AI, ms.Human
}

func (ms *MessageService) broadcastMessage(ctx context.Context, message []byte) {
	ms.mux.Lock()
	defer ms.mux.Unlock()
	for _, c := range ms.client {
		if err := c.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Println("broadcast message", err)
		}
	}
}
