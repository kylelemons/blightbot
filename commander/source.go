package commander

import (
	"github.com/kylelemons/blightbot/bot"
)

type Source struct {
	server  *bot.Server
	message *bot.Message
}

func (s *Source) Server() *bot.Server {
	return s.server
}

func (s *Source) Message() *bot.Message {
	return s.message
}

func (s *Source) ID() *bot.Identity {
	return s.message.ID()
}
