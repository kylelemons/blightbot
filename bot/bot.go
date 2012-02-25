package bot

import (
	"sync"
	"time"
)

type Handler func(event string, serv *Server, msg *Message)

type Bot struct {
	lock    sync.RWMutex
	id      *Identity
	servers []*Server

	ping    time.Duration
	timeout time.Duration

	LogLevel int

	callbacks map[string][]Handler
}

type Identity struct {
	Nick string
	User string
	Host string
}

func (id *Identity) String() string {
	if id.Nick == "" {
		if id.User == "" {
			return id.Host
		}
		return id.User + "@" + id.Host
	}
	return id.Nick + "!" + id.User + "@" + id.Host
}

func New(nick, user string) *Bot {
	return &Bot{
		LogLevel:  10,
		id:        &Identity{Nick: nick, User: user},
		ping:      60 * time.Second,
		timeout:   10 * time.Second,
		callbacks: map[string][]Handler{},
	}
}

func (b *Bot) SetPing(ping, timeout time.Duration) {
	b.ping, b.timeout = ping, timeout
}

func (b *Bot) OnConnect(h Handler) {
	b.OnEvent(ON_CONNECT, h)
}

func (b *Bot) OnDisconnect(h Handler) {
	b.OnEvent(ON_DISCONNECT, h)
}

func (b *Bot) OnEvent(event string, h Handler) {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.callbacks[event] = append(b.callbacks[event], h)
}
