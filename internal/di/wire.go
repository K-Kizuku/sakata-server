//go:build wireinject
// +build wireinject

package di

import (
	"github.com/K-Kizuku/sakata-server/internal/app/handler"
	"github.com/K-Kizuku/sakata-server/internal/app/repository"
	"github.com/K-Kizuku/sakata-server/internal/app/service"
	"github.com/K-Kizuku/sakata-server/pkg/ristretto"
	"github.com/google/wire"
)

func InitHandler() *handler.Root {
	wire.Build(
		ristretto.New,
		repository.NewMessageRepository,
		service.NewMessageService,
		handler.NewMessageHandler,
		handler.New,
	)
	return &handler.Root{}
}
