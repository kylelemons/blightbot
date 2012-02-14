package bot

import (
	"bytes"
	"strings"
)

// A Message represents a parsed line from the IRC server.
type Message struct {
	Prefix  string
	Command string
	Args    []string
}

// NewMessage creates a message with the given prefix, command, and arguments.
func NewMessage(pfx, cmd string, args ...string) *Message {
	return &Message{
		Prefix: pfx,
		Command: cmd,
		Args: args,
	}
}

// Copy copies the message.  This is a deep copy.
func (m Message) Copy() *Message {
	return &Message{
		Prefix: m.Prefix,
		Command: m.Command,
		Args: append(make([]string, 0, len(m.Args)), m.Args...),
	}
}

// ID returns the Identity of the sender of the message.
func (m *Message) ID() *Identity {
	id := &Identity{
		Host: m.Prefix,
	}

	uh := strings.SplitN(id.Host, "@", 2)
	if len(uh) != 2 { return id }
	id.User, id.Host = uh[0], uh[1]

	nu := strings.SplitN(id.User, "!", 2)
	if len(nu) != 2 { return id }
	id.Nick, id.User = nu[0], nu[1]

	return id
}

// ParseMessage returns the Message parsed from the given line.
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

// Bytes composes the message into a set of bytes for writing.
func (m *Message) Bytes() []byte {
	b := bytes.NewBuffer(make([]byte, 0, 128))
	// Write the message
	if len(m.Prefix) > 0 {
		b.WriteByte(':')
		b.WriteString(m.Prefix)
		b.WriteByte(' ')
	}
	b.WriteString(m.Command)
	for i, arg := range m.Args {
		b.WriteByte(' ')
		if i == len(m.Args)-1 && strings.IndexAny(arg, " :") >= 0 {
			// "escape" the long argument
			b.WriteByte(':')
		}
		b.WriteString(arg)
	}
	b.WriteByte('\n')
	return b.Bytes()
}

// String composes the message into a string.
func (m *Message) String() string {
	return string(m.Bytes())
}
