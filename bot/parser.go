package bot

import (
	"bytes"
	"strings"
)

type Message struct {
	Prefix  string
	Command string
	Args    []string
}

func NewMessage(pfx, cmd string, args []string) *Message {
	m := new(Message)
	m.Prefix = pfx
	m.Command = cmd
	m.Args = args
	return m
}

// Copy copies the message.  This is a deep copy.
func (m *Message) Copy() *Message {
	n := new(Message)
	*n = *m
	n.Args = append(make([]string, 0, len(m.Args)), m.Args...)
	return n
}

func ParseMessage(line string) *Message {
	line = strings.TrimSpace(line)
	if len(line) <= 0 {
		return nil
	}
	m := new(Message)
	if line[0] == ':' {
		split := strings.SplitN(line, " ", 2)
		if len(split) <= 1 {
			return nil
		}
		m.Prefix = string(split[0][1:])
		line = split[1]
	}
	split := strings.SplitN(line, ":", 2)
	args := strings.Split(strings.TrimSpace(split[0]), " ")
	m.Command = strings.ToUpper(args[0])
	m.Args = args[1:]
	if len(split) > 1 {
		m.Args = append(m.Args, string(split[1]))
	}
	return m
}

// Bytes builds the message and returns its bytes.  If longarg is
// true or there is a space in the last argument, it is prefixed
// with a colon.
func (m Message) Bytes(longarg bool) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	if len(m.Prefix) > 0 {
		buf.WriteByte(':')
		buf.WriteString(m.Prefix)
		buf.WriteByte(' ')
	}
	buf.WriteString(m.Command)
	for i, arg := range m.Args {
		buf.WriteByte(' ')
		if i == len(m.Args)-1 {
			if longarg || strings.IndexAny(arg, " :") >= 0 {
				buf.WriteByte(':')
			}
		}
		buf.WriteString(m.Args[i])
	}
	return buf.Bytes()
}

func (m Message) String() string {
	return string(m.Bytes(false))
}
