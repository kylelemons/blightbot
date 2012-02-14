package bot

import (
	"bufio"
	"fmt"
	"log"
	"io"
	"io/ioutil"
	"testing"
	"time"
)

type RW struct {
	io.Reader
	io.Writer
	io.Closer
}

func FakeConn() (RW, RW) {
	connR, connW := io.Pipe()
	fakeR, fakeW := io.Pipe()
	return RW{connR, fakeW, fakeW}, RW{fakeR, connW, connW}
}

type Send string
type Expect string
type EOF struct{}

var serverTests = []struct {
	Desc     string
	Bind     map[string]Handler
	Sequence []interface{}
}{
	{
		Desc: "err first",
		Sequence: []interface{}{
			Expect("NICK n"),
			Expect("USER u . . :blightbot-v0.0.0"),
			Send("ERROR :Some error"),
			Expect("QUIT :read closed"),
		},
	},
	{
		Desc: "eof",
		Sequence: []interface{}{
			Expect("NICK n"),
			Expect("USER u . . :blightbot-v0.0.0"),
			EOF{},
			Expect("QUIT :read closed"),
		},
	},
	{
		Desc: "onconn",
		Bind: map[string]Handler{
			ON_CONNECT: func(e string, s *Server, m *Message) {
				s.WriteString("JOIN #chan\n")
			},
		},
		Sequence: []interface{}{
			Expect("NICK n"),
			Expect("USER u . . :blightbot-v0.0.0"),
			Send(":serv 001 :Welcome"),
			Expect("JOIN #chan"),
			EOF{},
			Expect("QUIT :read closed"),
		},
	},
	{
		Desc: "onconn",
		Bind: map[string]Handler{
			ON_DISCONNECT: func(e string, s *Server, m *Message) {
				s.WriteString("DC\n")
			},
		},
		Sequence: []interface{}{
			Expect("NICK n"),
			Expect("USER u . . :blightbot-v0.0.0"),
			EOF{},
			Expect("QUIT :read closed"),
			Expect("DC"),
		},
	},
	{
		Desc: "collide",
		Sequence: []interface{}{
			Expect("NICK n"),
			Expect("USER u . . :blightbot-v0.0.0"),
			Send(":serv 433 nk :Nickname already in use"),
			Expect("NICK nk_"),
			EOF{},
			Expect("QUIT :read closed"),
		},
	},
	{
		Desc: "collide bad 443",
		Sequence: []interface{}{
			Expect("NICK n"),
			Expect("USER u . . :blightbot-v0.0.0"),
			Send(":serv 433 :Nickname already in use"),
			Expect("NICK n_"),
			EOF{},
			Expect("QUIT :read closed"),
		},
	},
	{
		Desc: "joinpart",
		Bind: map[string]Handler{
			ON_CONNECT: func(e string, s *Server, m *Message) {
				fmt.Fprintf(s, "JOIN #test\n")
			},
			ON_JOIN: func(e string, s *Server, m *Message) {
				fmt.Fprintf(s, "PRIVMSG %s :Hello!\n", m.Args[0])
				fmt.Fprintf(s, "PART %s :later\n", m.Args[0])
			},
			ON_PART: func(e string, s *Server, m *Message) {
				fmt.Fprintf(s, "NOTICE @%s :Laterz\n", m.Args[0])
			},
		},
		Sequence: []interface{}{
			Expect("NICK n"),
			Expect("USER u . . :blightbot-v0.0.0"),
			Send(":serv 001 :Welcome"),
			Expect("JOIN #test"),
			Send(":n!u@h JOIN :#test"),
			Expect("PRIVMSG #test :Hello!"),
			Expect("PART #test :later"),
			Send(":n!u@h PART #test :later"),
			Expect("NOTICE @#test :Laterz"),
			EOF{},
			Expect("QUIT :read closed"),
		},
	},
}

func TestServer(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	for _, test := range serverTests {
		desc := test.Desc
		done := make(chan bool)

		go func() {
			bot := New("n", "u")
			conn, local := FakeConn()
			bot.newServer("s:p", conn)

			fake := bufio.NewReader(local)

			for evt, fun := range test.Bind {
				bot.OnEvent(evt, fun)
			}

			after := "connect"
			for _, seq := range test.Sequence {
				switch act := seq.(type) {
				case EOF:
					local.Close()
				case Send:
					io.WriteString(local, string(act)+"\n")
					after = string(act)
				case Expect:
					line, err := fake.ReadString('\n')
					if err != nil {
						t.Errorf("%s: unexpected error: %s", desc, err)
					}
					if got, want := line, string(act)+"\n"; got != want {
						t.Errorf("%s: got %q, want %q [after %q]", desc, got, want, after)
					}
				}
			}
			for {
				line, err := fake.ReadString('\n')
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("%s: unexpected late error: %s", desc, err)
					break
				}
				t.Errorf("%s: unexpected line %q", desc, line)
			}
			done <- true
		}()
		select {
		case <-done:
		case <-time.After(1e9):
			t.Errorf("%s: timed out", desc)
		}
	}
}
