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
	pass    = flag.String("pass", "", "Server password to use")
	nsid    = flag.String("identify", "", "Nickserv password with which to identify")
	server  = flag.String("server", "irc.freenode.net:6667", "Server to which the bot should connect")
	channel = flag.String("channel", "#ircd-blight", "Channel(s) to join (commas, no spaces)")
)

func OnConnect(event string, serv *bot.Server, msg *bot.Message) {
	if *nsid != "" {
		serv.WriteMessage(bot.NewMessage("", bot.CMD_PRIVMSG, "NickServ", "IDENTIFY " + *nsid))
		time.Sleep(3)
	}
	serv.WriteMessage(bot.NewMessage("", "JOIN", *channel))
}

func OnDisconnect(event string, serv *bot.Server, msg *bot.Message) {
	log.Printf("Server disconnected.")
	for {
		log.Printf("Recnnecting in 60s...")
		time.Sleep(60*time.Second)
		if err := serv.Bot().ConnectPass(*server, *pass); err != nil {
			log.Printf("connect: %s", err)
			continue
		}
		break
	}
}

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	b := bot.New(*nick, *user)
	b.OnConnect(OnConnect)
	b.OnDisconnect(OnDisconnect)
	if err := b.ConnectPass(*server, *pass); err != nil {
		log.Fatalf("connect: %s", err)
	}
	log.Printf("Bot is running...")
	commander.Run(b, '!', []*commander.Command{
		gonuts.Issue,
		gonuts.CL,
	})
}
