package service

import (
	"context"
	"encoding/json"
	"iter"
	"log"
	"sync"
	"time"

	"github.com/K-Kizuku/sakata-server/internal/app/repository"
	"github.com/gorilla/websocket"
)

type IMessageService interface {
	ReadPump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{})
	WritePump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{})
	JoinRoom(ctx context.Context, conn *websocket.Conn, roomID string) error
	LeaveRoom(ctx context.Context, conn *websocket.Conn, roomID string) error
}

type MessageService struct {
	mr   repository.IMessageRepository
	room map[string][]*websocket.Conn
	mux  sync.RWMutex
}

func NewMessageService(mr repository.IMessageRepository) IMessageService {
	return &MessageService{mr: mr}
}

func (ms *MessageService) JoinRoom(ctx context.Context, conn *websocket.Conn, roomID string) error {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	if _, ok := ms.room[roomID]; ok {
		ms.room[roomID] = append(ms.room[roomID], conn)
	} else {
		ms.room[roomID] = make([]*websocket.Conn, 0, 10) // 10人くらいを想定
		ms.room[roomID] = append(ms.room[roomID], conn)
	}
	return nil
}

func (ms *MessageService) LeaveRoom(ctx context.Context, conn *websocket.Conn, roomID string) error {
	ms.mux.Lock()
	defer ms.mux.Unlock()

	if _, ok := ms.room[roomID]; ok {
		for i, c := range ms.room[roomID] {
			if c == conn {
				ms.room[roomID] = append(ms.room[roomID][:i], ms.room[roomID][i+1:]...)
				break
			}
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
			timestamp := time.Now().Unix()
			if err := ms.mr.AddMessage(ctx, timestamp, string(message)); err != nil {
				log.Println("add message")

				errCh <- err
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Println("write message")
				errCh <- err
				return
			}
		}
	}
}

type Messages struct {
	Messages map[string]int `json:"messages"`
}

func (ms *MessageService) WritePump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{}) {
	defer conn.Close()
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			doneCh <- struct{}{}
			return
		case <-ticker.C:
			now := time.Now().Unix()

			messages, err := ms.mr.GetMessagesByTimeRange(ctx, now, 15)
			// messages, err := ms.mr.GetMessagesFromMock(ctx, now, 5)
			if err != nil {
				log.Println("Redisからのデータ取得エラー:", err)
				errCh <- err
				return
			}

			// messageCount := make(map[string]int)
			// for _, message := range messages {
			// 	messageCount[message]++
			// }
			m, err := json.Marshal(Messages{Messages: messages})
			if err != nil {
				errCh <- err
				return
			}
			err = conn.WriteMessage(websocket.TextMessage, m)
			if err != nil {
				errCh <- err
				return
			}
		}
	}
}

func (ms *MessageService) getClientByRoomID(id string) iter.Seq[*websocket.Conn] {
	ms.mux.Lock()
	defer ms.mux.Unlock()
	return func(yield func(*websocket.Conn) bool) {
		for _, v := range ms.room[id] {
			for !yield(v) {
				return
			}
		}
	}
}
