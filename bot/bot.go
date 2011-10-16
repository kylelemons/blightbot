package bot

import (
	"sync"
)

type Bot struct {
	lock     sync.Mutex
	id       *Identity
	servers  []*Server
}

type Identity struct {
	Nick string
	User string
	Host string
}

func New(name string) *Bot {
	return &Bot{
		id: &Identity{Nick: name},
	}
}
