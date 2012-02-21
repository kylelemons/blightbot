package bot

import (
	"testing"
)

var messageTests = []struct {
	Prefix, Cmd string
	Args        []string
	Expect      string
	ForceLong   bool
}{
	{
		Prefix: "server.kevlar.net",
		Cmd:    "NOTICE",
		Args:   []string{"user", "*** This is a test"},
		Expect: ":server.kevlar.net NOTICE user :*** This is a test\n",
	},
	{
		Prefix: "A",
		Cmd:    "B",
		Args:   []string{"C"},
		Expect: ":A B C\n",
	},
	{
		Cmd:    "B",
		Args:   []string{"C"},
		Expect: "B C\n",
	},
	{
		Prefix: "A",
		Cmd:    "B",
		Args:   []string{"C", "D"},
		Expect: ":A B C D\n",
	},
}

func TestBuildMessage(t *testing.T) {
	for i, test := range messageTests {
		m := &Message{
			Prefix:  test.Prefix,
			Command: test.Cmd,
			Args:    test.Args,
		}
		if got, want := m.String(), test.Expect; got != want {
			t.Errorf("%d. string = %q, want %q", i, got, want)
		}
	}
}

func TestParseMesage(t *testing.T) {
	for i, test := range messageTests {
		m := ParseMessage(test.Expect)
		if test.Prefix != m.Prefix {
			t.Errorf("#d. prefix = %q, want %q", i, m.Prefix, test.Prefix)
		}
		if test.Cmd != m.Command {
			t.Errorf("#d. command = %q, want %q", i, m.Command, test.Cmd)
		}
		if len(test.Args) != len(m.Args) {
			t.Errorf("#d. args = %v, want %v", i, m.Args, test.Args)
		} else {
			for j := 0; j < len(test.Args) && j < len(m.Args); j++ {
				if test.Args[j] != m.Args[j] {
					t.Errorf("#d. arg[%d] = %q, want %q", i, m.Args[j], test.Args[j])
				}
			}
		}
	}
}

var parseBench = ":irc.example.com NOTICE user :*** This is a test"

func BenchmarkParseMessage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseMessage(parseBench)
	}
}

var buildBench = ParseMessage(parseBench)

func BenchmarkMessageString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buildBench.String()
	}
}

func BenchmarkMessageBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buildBench.Bytes()
	}
}
