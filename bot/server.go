package bot

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	ReadChannelBuffer = 32
)

type Server struct {
	bot  *Bot
	id   *Identity
	name string
	pass string
	conn io.ReadWriteCloser
	pong chan bool

	lock     sync.RWMutex
	channels map[string]*Channel

	inc chan *Message
}

func (s *Server) Bot() *Bot     { return s.bot }
func (s *Server) ID() *Identity { return s.id }
func (s *Server) Name() string  { return s.name }

func (s *Server) Me(id *Identity) bool {
	return id.Nick == s.id.Nick
}

func (b *Bot) newServer(name, pass string, rwc io.ReadWriteCloser) {
	s := &Server{
		bot:      b,
		id:       b.id,
		name:     name,
		pass:     pass,
		pong:     make(chan bool),
		conn:     rwc,
		inc:      make(chan *Message, 32),
		channels: map[string]*Channel{},
	}

	b.lock.Lock()
	defer b.lock.Unlock()
	b.servers = append(b.servers, s)

	go s.manage()
	go s.reader()
	go s.pingloop()
}

func (b *Bot) Connect(server string) error {
	addr, err := net.ResolveTCPAddr("tcp", server)
	if err != nil {
		return err
	}

	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return err
	}

	// Enable TCP keepalives
	conn.SetKeepAlive(true)

	b.newServer(server, "", conn)
	return nil
}

func (b *Bot) ConnectPass(server, pass string) error {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return err
	}
	b.newServer(server, pass, conn)
	return nil
}

func (s *Server) pingloop() {
	defer func() {
		s.conn.Close()
	}()

	ping, timeout := s.bot.ping, s.bot.timeout

	message := []byte("PING :blight-bot\n")
	for {
		time.Sleep(ping)
		if _, err := s.conn.Write(message); err != nil {
			log.Printf("ping: %s")
			return
		}
		select {
		case <-s.pong:
		case <-time.After(timeout):
			io.WriteString(s.conn, "QUIT :ping time exceeded\n")
			time.Sleep(1 * time.Second)
			return
		}
	}
}

func (s *Server) manage() {
	defer s.conn.Close()
	defer s.trigger(ON_DISCONNECT, nil)
	if s.pass != "" {
		fmt.Fprintf(s.conn, "PASS %s\n", s.pass)
	}
	fmt.Fprintf(s.conn, "NICK %s\nUSER %s . . :%s\n",
		s.id.Nick, s.id.User, "github.com/kylelemons/blightbot-v0.0.0")
	for {
		select {
		case inc, ok := <-s.inc:
			if !ok {
				io.WriteString(s.conn, "QUIT :read closed\n")
				return
			}
			if s.bot.LogLevel > 3 {
				log.Printf(">> %s", inc)
			}
			switch inc.Command {
			case RPL_WELCOME:
				s.trigger(ON_CONNECT, inc)
				if len(inc.Args) > 0 {
					s.id.Nick = inc.Args[0]
				}
			case ERR_NICKNAMEINUSE:
				nick := s.id.Nick
				if len(inc.Args) > 1 {
					nick = inc.Args[0]
				}
				nick += "_"
				fmt.Fprintf(s.conn, "NICK %s\n", nick)
			case CMD_JOIN:
				if len(inc.Args) < 1 {
					break
				}
				channame := inc.Args[0]

				user := inc.ID()
				if !s.Me(user) {
					break
				}
				s.newChannel(channame)

				s.trigger(ON_JOIN, inc)
			case CMD_PART:
				if len(inc.Args) < 1 {
					break
				}
				channame := inc.Args[0]

				user := inc.ID()
				if !s.Me(user) {
					break
				}
				s.delChannel(channame)

				s.trigger(ON_PART, inc)
			case CMD_PING:
				s.WriteMessage(NewMessage("", "PONG", inc.Args...))
			case CMD_PONG:
				s.pong <- true
			case CMD_PRIVMSG:
				if len(inc.Args) < 2 {
					break
				}
				var private, channel bool
				for _, target := range strings.Split(inc.Args[0], ",") {
					if len(target) > 0 && target[0] == '#' {
						channel = true
					}
					if target == s.id.Nick {
						private = true
					}
				}
				if channel {
					s.trigger(ON_CHANMSG, inc)
				} else if private {
					s.trigger(ON_PRIVMSG, inc)
				}
			case CMD_NOTICE:
				if len(inc.Args) < 2 {
					break
				}
				var private, channel bool
				for _, target := range strings.Split(inc.Args[0], ",") {
					if len(target) > 0 && target[0] == '#' {
						channel = true
					}
					if target == s.id.Nick {
						private = true
					}
				}
				if channel {
					// Ignore channel notices
				} else if private {
					s.trigger(ON_NOTICE, inc)
				}
			}
		}
	}
}

func (s *Server) reader() {
	defer close(s.inc)
	in := bufio.NewReader(s.conn)
	for {
		line, err := in.ReadString('\n')
		if err == io.EOF {
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

func (s *Server) trigger(event string, m *Message) {
	s.bot.lock.RLock()
	defer s.bot.lock.RUnlock()

	if s.bot.LogLevel > 0 {
		log.Printf("Trigger: %s | %s", event, m)
	}

	for _, f := range s.bot.callbacks[event] {
		go f(event, s, m)
	}
}

func (s *Server) Write(b []byte) (int, error) {
	return s.conn.Write(b)
}

func (s *Server) WriteString(str string) (int, error) {
	return io.WriteString(s.conn, str)
}

func (s *Server) WriteMessage(m *Message) (int, error) {
	log.Printf("<< %s", m)
	return s.conn.Write(m.Bytes())
}
