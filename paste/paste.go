package paste

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/kylelemons/blightbot/bot"
	"github.com/kylelemons/blightbot/commander"
	"github.com/kylelemons/gopaste/subscribe"
)

var (
	chans = flag.String("paste-chans", "", "Channels to send paste notifications on")
)

func nopaste(s *commander.Source, r *commander.Response, cmd string, args []string) {
	r.Public()
	r.Printf("If you need to paste more than 3 lines, use gp: go get github.com/kylelemons/gopaste/gp")
}

var NoPaste = commander.Cmd("nopaste", nopaste).Help(`Print a message pointing to the gopaste tool`)

var servers = struct {
	sync.Mutex
	m map[string]*bot.Server
}{m: map[string]*bot.Server{}}

func addServer(event string, serv *bot.Server, msg *bot.Message) {
	servers.Lock()
	defer servers.Unlock()

	servers.m[serv.Name()] = serv
}

func delServer(event string, serv *bot.Server, msg *bot.Message) {
	servers.Lock()
	defer servers.Unlock()

	delete(servers.m, serv.Name())
}

func Register(b *bot.Bot) {
	if len(*chans) == 0 {
		log.Printf("skipping paste init: no channels to notify")
		return
	}

	b.OnConnect(addServer)
	b.OnDisconnect(delServer)

	go pasteloop()
}

func pasteloop() {
	chans := strings.Split(*chans, ",")

	random := make([]byte, 18)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		panic("not enough entropy")
	}
	clientID := base64.URLEncoding.EncodeToString(random)

	send := func(url string) {
		servers.Lock()
		defer servers.Unlock()

		msg := &bot.Message{
			Command: bot.CMD_PRIVMSG,
			Args: []string{
				"",
				fmt.Sprintf("pasted: %s", url),
			},
		}
		for sname, srv := range servers.m {
			for _, channel := range chans {
				log.Printf("Writing to %s on %s", channel, sname)
				msg.Args[0] = channel
				srv.WriteMessage(msg)
			}
		}
	}

	for {
		log.Printf("Starting paste subscriber...")
		urls := make(chan string)
		go func() {
			if err := subscribe.Subscribe(clientID, urls); err != nil {
				log.Printf("subscribe: %s", err)
			}
		}()
		for url := range urls {
			send(url)
		}
	}
}
