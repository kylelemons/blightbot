package bot

import (
	"bytes"
	"io"
	"os"
	"testing"
	"strings"
)

type FakeConn struct {
	io.Reader
	io.Writer
	done chan bool
}

func NewFake() FakeConn { return FakeConn{new(bytes.Buffer), new(bytes.Buffer), make(chan bool, 1)} }
func (c FakeConn) Close() os.Error { c.done <- true; return nil }

func (c FakeConn) PutInput(raw string) {
	io.WriteString(c.Reader.(*bytes.Buffer), strings.TrimSpace(raw) + "\n")
}
func (c FakeConn) CompareOutput(raw string) (bool, string, string) {
	got, want := c.Writer.(*bytes.Buffer).String(),  strings.TrimSpace(raw) + "\n"
	return got == want, got, want
}
func (c FakeConn) Wait() {
	<-c.done
}

var serverTests = []struct{
	Desc   string
	Input  string
	Output string
}{
	{
		Desc: "err first",
		Input: `
ERROR :Some error
`,
		Output: `
QUIT :read closed
`,
	},
}

func TestServer(t *testing.T) {
	for _, test := range serverTests {
		desc := test.Desc
		conn := NewFake()
		b := New("bot")
		b.newServer("serv:port", conn)
		conn.PutInput(test.Input)
		conn.Wait()
		if match, got, want := conn.CompareOutput(test.Output); !match {
			t.Errorf("==== %s ====\nGOT:\n%s----\nWANT:\n%s", desc, got, want)
		}
	}
}
