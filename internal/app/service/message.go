package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/K-Kizuku/sakata-server/internal/app/repository"
	"github.com/gorilla/websocket"
)

type IMessageService interface {
	ReadPump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{})
	WritePump(ctx context.Context, conn *websocket.Conn, errCh chan<- error, doneCh chan<- struct{})
}

type MessageService struct {
	mr repository.IMessageRepository
}

func NewMessageService(mr repository.IMessageRepository) IMessageService {
	return &MessageService{mr: mr}
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
			log.Println("直近5秒間のメッセージ集計:", messages)
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
