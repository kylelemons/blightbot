package bot

import (
	"sync"
)

type Channel struct {
	serv *Server
	lock sync.RWMutex

	name string
}

func (s *Server) newChannel(name string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.channels[name] = &Channel{serv: s, name: name}
}

func (s *Server) delChannel(name string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.channels, name)
}

func (s *Server) GetChannel(name string) *Channel {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.channels[name]
}
