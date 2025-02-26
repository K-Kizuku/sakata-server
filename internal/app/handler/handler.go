package handler

type Root struct {
	MessageHandler IMessageHandler
}

func New(MessageHandler IMessageHandler) *Root {
	return &Root{
		MessageHandler: MessageHandler,
	}
}
