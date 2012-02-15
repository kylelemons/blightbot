package main

import (
	"flag"
	"log"
	"fmt"
	"math/rand"
	"time"

	"github.com/kylelemons/blightbot/bot"
	"github.com/kylelemons/blightbot/commander"
	"github.com/kylelemons/blightbot/gonuts"
)

func randname() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("BlightBot%04d", rand.Intn(10000))
}

var (
	nick    = flag.String("nick", randname(), "Nick to use when connecting")
	user    = flag.String("user", "blight", "Username to use when connecting")
	server  = flag.String("server", "irc.freenode.net:6667", "Server to which the bot should connect")
	channel = flag.String("channel", "#ircd-blight", "Channel to join")
)

func OnConnect(event string, serv *bot.Server, msg *bot.Message) {
	serv.WriteMessage(bot.NewMessage("", "JOIN", *channel))
}

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	b := bot.New(*nick, *user)
	b.OnConnect(OnConnect)
	if err := b.Connect(*server); err != nil {
		log.Fatalf("connect: %s", err)
	}
	log.Printf("Bot is running...")
	commander.Run(b, '!', []*commander.Command{
		gonuts.Issue,
	})
}
