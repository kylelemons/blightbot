package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/kylelemons/blightbot/acro"
	"github.com/kylelemons/blightbot/bot"
	"github.com/kylelemons/blightbot/commander"
	"github.com/kylelemons/blightbot/gonuts"
	"github.com/kylelemons/blightbot/paste"
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
	server  = flag.String("servers", "irc.freenode.net:6667", "Servers (addr:port) to which the bot should connect")
	channel = flag.String("channels", "#ircd-blight,#acrogame", "Channel(s) to join (commas, no spaces)")
	delay   = flag.Duration("delay", 5*time.Second, "Delay after disconnect")
	rdelay  = flag.Duration("reconnect-wait", 60*time.Second, "Time to wait before reconnecting after a failed connection")
	modules = flag.String("modules", "", "Comma separated list of modules to load: "+modlist())
)

var servers = map[string]string{}

var modlists = map[string][]*commander.Command{
	"gonuts": {
		gonuts.Issue,
		gonuts.CL,
		gonuts.Doc,
		gonuts.EGo,
		gonuts.FAQ,
		gonuts.Go1,
		gonuts.Compat,
		gonuts.Pkg,
		gonuts.Cmd,
		gonuts.Spec,
		gonuts.TPDoc,
	},
	"acro": {
		acro.Acro,
	},
	"paste": {
		paste.NoPaste,
	},
}

func modlist() string {
	list := []string{}
	for mod := range modlists {
		list = append(list, mod)
	}
	return strings.Join(list, " ")
}

func OnConnect(event string, serv *bot.Server, msg *bot.Message) {
	if *nsid != "" {
		serv.WriteMessage(bot.NewMessage("", bot.CMD_PRIVMSG, "NickServ", "IDENTIFY "+*nsid))
		time.Sleep(3)
	}
	serv.WriteMessage(bot.NewMessage("", "JOIN", *channel))
}

func OnDisconnect(event string, serv *bot.Server, msg *bot.Message) {
	server, pass := serv.Name(), servers[serv.Name()]
	log.Printf("Server %q disconnected.", server)
	time.Sleep(*delay)
	for {
		log.Printf("Recnnecting to %q...", server)
		if err := serv.Bot().ConnectPass(server, pass); err != nil {
			log.Printf("connect: %s", err)
			time.Sleep(*rdelay)
			continue
		}
		break
	}
}

func main() {
	flag.Parse()

	// Parse servers
	s, p := strings.Split(*server, ","), strings.Split(*pass, ",")
	for i, serv := range s {
		var pass string
		if i < len(p) {
			pass = p[i]
		}
		servers[serv] = pass
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	b := bot.New(*nick, *user)
	b.OnConnect(OnConnect)
	b.OnDisconnect(OnDisconnect)

	var cmds []*commander.Command
	for _, mod := range strings.Split(*modules, ",") {
		list, ok := modlists[mod]
		if !ok {
			continue
		}
		log.Printf("Loading commands from %q", mod)
		cmds = append(cmds, list...)

		switch mod {
		case "gonuts":
			log.Printf("Starting godoc polling")
			gonuts.StartPolling()
		case "paste":
			log.Printf("Initializing paste module")
			paste.Register(b)
		}
	}
	go commander.Run(b, '!', cmds)

	for server, pass := range servers {
		log.Printf("Connecting to %q...", server)
		if err := b.ConnectPass(server, pass); err != nil {
			log.Fatalf("connect: %s", err)
		}
	}

	log.Printf("Bot is running...")
	select {}
}
