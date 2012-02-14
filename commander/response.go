package commander

import (
	"fmt"
	"strings"

	"blightbot/bot"
)

type Response struct {
	// Reply channel
	out chan *bot.Message

	// Possible settings
	public  string
	private string

	// The current setting
	target string
	msgtyp string
}

func (r *Response) Public() {
	r.target, r.msgtyp = r.public, bot.CMD_PRIVMSG
}

func (r *Response) Private() {
	r.target, r.msgtyp = r.private, bot.CMD_NOTICE
}

func (r *Response) WriteString(s string) {
	if r.target == "" {
		return
	}
	r.out <- bot.NewMessage("", r.msgtyp, r.target, s)
}

func (r *Response) Printf(format string, args ...interface{}) {
	if r.target == "" {
		return
	}
	r.out <- bot.NewMessage("", r.msgtyp, r.target, fmt.Sprintf(format, args...))
}

func (r *Response) done() {
	close(r.out)
}

func Bold(s string) string {
	return "\x02" + s + "\x0F"
}

func Underline(s string) string {
	return "\x16" + s + "\x0F"
}

func DecodeCTCP(s string) string {
	// Strip leading CTCP char
	if s[0] == 0x01 {
		s = s[1:]
	}
	if len(s) == 0 {
		return s
	}

	// Strip trailing CTCP char
	if s[len(s)-1] == 0x01 {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		return s
	}

	// Decode text
	if strings.IndexRune(s, 0x10) >= 0 {
		s = strings.Replace(s, "\x100", "\x00", -1)
		s = strings.Replace(s, "\x10r", "\r", -1)
		s = strings.Replace(s, "\x10n", "\n", -1)
		s = strings.Replace(s, "\x10\x10", "\x10", -1)
		s = strings.Replace(s, "\x10", "", -1)
	}
	return s
}

func EncodeCTCP(s string) string {
	s = strings.Replace(s, "\x00", "\x100",    -1)
	s = strings.Replace(s, "\r",   "\x10r",    -1)
	s = strings.Replace(s, "\n",   "\x10n",    -1)
	s = strings.Replace(s, "\x10", "\x10\x10", -1)
	return "\x01" + s + "\x01"
}
