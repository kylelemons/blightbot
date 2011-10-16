package bot

import (
	"bufio"
	"io"
	"log"
	"net"
	"os"
)

const (
	ReadChannelBuffer = 32
)

type Server struct {
	bot  *Bot
	id   *Identity
	name string
	conn io.ReadWriteCloser

	inc chan *Message
}

func (s *Server) Bot() *Bot    { return s.bot }
func (s *Server) ID() Identity { return *s.id }
func (s *Server) Name() string { return s.name }

func (b *Bot) newServer(name string, rwc io.ReadWriteCloser) {
	s := &Server{
		name: name,
		conn: rwc,
		inc:  make(chan *Message, 32),
	}

	b.lock.Lock()
	defer b.lock.Unlock()
	b.servers = append(b.servers, s)

	go s.manage()
	go s.reader()
}

func (b *Bot) Connect(server string) os.Error {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return err
	}
	b.newServer(server, conn)
	return nil
}

func (s *Server) manage() {
	defer s.conn.Close()
	for {
		select {
		case inc, ok := <-s.inc:
			if !ok {
				io.WriteString(s.conn, "QUIT :read closed\n")
				return
			}
			_ = inc
		}
	}
}

func (s *Server) reader() {
	defer close(s.inc)
	in := bufio.NewReader(s.conn)
	for {
		line, err := in.ReadString('\n')
		if err == os.EOF {
			s.Log("EOF")
			return
		}
		if err != nil {
			s.Log("read: %s", err)
			return
		}

		msg := ParseMessage(line)
		if msg == nil {
			continue
		}

		if msg.Command == CMD_ERROR {
			s.Log("ERROR %v", msg.Args)
			return
		}

		s.inc <- msg
	}
}

func (s *Server) Log(format string, args ...interface{}) {
	log.Printf("["+s.name+"] "+format, args...)
}
