package paste

import (
	"flag"
	"github.com/kylelemons/blightbot/bot"
	"github.com/kylelemons/blightbot/commander"
	"github.com/kylelemons/go-rpcgen/webrpc"
	"github.com/kylelemons/gopaste/proto"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	chans = flag.String("paste-chans", "", "Channels to send paste notifications on")
	psrvurl = flag.String("paste-server", "http://paste.kylelemons.net:4114/", "Server to query for pastes")
)

func nopaste(s *commander.Source, r *commander.Response, cmd string, args []string) {
	r.Public()
	r.Printf("If you need to paste more than 3 lines, use gopaste: https://github.com/kylelemons/gopaste/")
}

var NoPaste = commander.Cmd("nopaste", nopaste).Help(`Print a message pointing to the gopaste tool`)

var servers = map[string]*bot.Server{}
var servlock sync.Mutex

func addServer(event string, serv *bot.Server, msg *bot.Message) {
	servlock.Lock()
	defer servlock.Unlock()

	servers[serv.Name()] = serv
}

func delServer(event string, serv *bot.Server, msg *bot.Message) {
	servlock.Lock()
	defer servlock.Unlock()

	delete(servers, serv.Name())
}

func Register(b *bot.Bot) {
	if len(*chans) == 0 {
		log.Printf("skipping paste init: no channels to notify")
	}

	b.OnConnect(addServer)
	b.OnDisconnect(delServer)

	url, err := url.Parse(*psrvurl)
	if err != nil {
		log.Fatalf("paste: bad url %q: %s", *psrvurl, err)
	}

	psrv := proto.NewGoPasteWebClient(webrpc.ProtoBuf, url)
	go waitloop(psrv)
}

func waitloop(psrv proto.GoPaste) {
	chans := strings.Split(*chans, ",")
	in, out := proto.Empty{}, proto.Posted{}

	var (
		errdelay = 100*time.Millisecond
		startdelay = 100*time.Millisecond
		maxdelay = 10*time.Minute
	)
	for {
		err := psrv.Next(&in, &out)
		if err != nil {
			log.Printf("paste: next URL: %s", err)
			time.Sleep(errdelay)
			errdelay *= 2
			if errdelay > maxdelay {
				errdelay = maxdelay
			}
			continue
		}
		errdelay = startdelay

		if out.Url == nil {
			log.Printf("paste: warning: empty URL")
			continue
		}
		

		func(){
			servlock.Lock()
			defer servlock.Unlock()

			for _, cname := range chans {
				msg := bot.NewMessage("", bot.CMD_PRIVMSG, cname, "gopaste: "+ *out.Url)
				for sname, s := range servers {
					log.Printf("paste: sending paste to %s on %s", cname, sname)
					s.WriteMessage(msg)
				}
			}
		}()
	}
}
