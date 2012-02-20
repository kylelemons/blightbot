package acro

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kylelemons/blightbot/bot"
	"github.com/kylelemons/blightbot/commander"
)

var (
	acrostart = flag.Duration("acro-start", 1*time.Minute, "Acro start delay")
	acrosubmit = flag.Duration("acro-submit", 2*time.Minute, "Acro submission time")
	acrovote = flag.Duration("acro-vote", 1*time.Minute, "Acro vote time")
	acromin = flag.Int("acro-minlen", 4, "Acro minimum acronym")
	acromax = flag.Int("acro-maxlen", 6, "Acro maximum acronym")
)

func gen() string {
	const choose = "AAAAABBBBCCCDDDEEEEEEFFFGGGGHHHIIIIIJJJKKLLLLLMMMMMMNNNOOOOOPPQRRRSSSSSSTTTUUVVVWXYYZ"
	letters := make([]byte, *acromin + rand.Intn(*acromax-*acromin+1))
	for i := range letters {
		letters[i] = choose[rand.Intn(len(choose))]
	}
	return string(letters)
}

func firstletters(acro string) string {
	if acro == "" {
		return ""
	}

	words := strings.Fields(strings.ToUpper(acro))
	letters := make([]byte, len(words))
	for i, word := range words {
		letters[i] = word[0]
	}
	return string(letters)
}

type Game struct {
	server  *bot.Server
	channel string

	started  bool
	commands chan gameCommand
}

type gameCommand interface {
	writef(string, ...interface{})
	done()
}

func (g *Game) Chanf(format string, args ...interface{}) {
	g.server.WriteMessage(bot.NewMessage("", "PRIVMSG", g.channel, fmt.Sprintf(format, args...)))
}

func (g *Game) String() string {
	return g.server.Name() + "/" + g.channel
}

func (g *Game) start() {
	g.started = true
	g.commands = make(chan gameCommand)

	go func() {
		defer func() {
			g.started = false

			// Just to be safe, grab any lingering commands for 60s
			go func() {
				reallydone := time.After(60*time.Second)
				for {
					select {
					case cmd := <-g.commands:
						log.Printf("Lingering acro command: %#v", cmd)
						cmd.done()
					case <-reallydone:
						return
					}
				}
			}()
		}()

		botnick := g.server.ID().Nick

		g.Chanf(`Acro is starting in %s! Type "!acro join" or "/msg %s ACRO %s JOIN" to join!`,
			*acrostart, botnick, g.channel)

		joinstop := time.After(*acrostart)
		players := map[string]string{}
		joins:
		for {
			select {
			case <-joinstop:
				break joins
			case cmd := <-g.commands:
				switch j := cmd.(type) {
				case *join:
					if _, ok := players[j.nick]; ok {
						cmd.writef("You've alredy joined.  Submissions start soon!")
						break
					}
					players[j.nick] = ""
					g.Chanf("%s has joined the game!", j.nick)
				default:
					cmd.writef("Sorry, it's the join phase now")
				}
				cmd.done()
			}
		}

		if len(players) < 3 {
			g.Chanf("You can't play Acro with fewer than 3 players!")
			return
		}

		acro := gen()
		g.Chanf(`Your acro this round is: %s`, commander.Bold(acro))
		g.Chanf(`Type "/msg %s ACRO %s SUBMIT <acronym>" in the next %s to submit!`,
			botnick, g.channel, *acrosubmit)

		submitstop := time.After(*acrosubmit)
		submits:
		for {
			select {
			case <-submitstop:
				break submits
			case cmd := <-g.commands:
				switch s := cmd.(type) {
				case *submission:
					if _, ok := players[s.nick]; !ok {
						cmd.writef("You didn't join the game!")
						break
					}
					if user := firstletters(s.acro); user != acro {
						cmd.writef("Your acronym doesn't match! It spells %s, not %s.", user, acro)
						break
					}
					players[s.nick] = s.acro
					cmd.writef("Acronym accepted!")
				default:
					cmd.writef("Sorry, it's time to submit acronyms now!")
				}
				cmd.done()
			}
		}

		subcnt := 0
		for _, sub := range players {
			if sub != "" {
				subcnt++
			}
		}
		if subcnt < 2 {
			g.Chanf("Sorry, we need more than one acronym to vote!")
			return
		}

		votes := make([]int, 0, len(players))
		playerIndex := make([]string, 0, len(players))
		for player, submitted := range players {
			if submitted == "" {
				continue
			}
			votes = append(votes, 0)
			playerIndex = append(playerIndex, player)
			g.Chanf("%d. %s", len(votes), submitted)
		}
		g.Chanf(`Type "/msg %s ACRO %s VOTE <number>" in the next %s to vote!`,
			botnick, g.channel)

		voted := map[string]bool{}
		votestop := time.After(*acrovote)
		votes:
		for {
			select {
			case <-votestop:
				break votes
			case cmd := <-g.commands:
				switch s := cmd.(type) {
				case *vote:
					if voted[s.nick] {
						cmd.writef("You already voted!")
						break
					}
					if s.idx < 0 || s.idx >= len(votes) {
						cmd.writef("Invalid vote!")
						break
					}
					voted[s.nick] = true
					votes[s.idx]++
					cmd.writef("Your vote has been counted.")
				default:
					cmd.writef("Sorry, it's time to vote now!")
				}
				cmd.done()
			}
		}

		if len(voted) < 1 {
			g.Chanf("Everybody wins! (because nobody voted...)")
			return
		}

		sort.Sort(votesort{votes, playerIndex})

		g.Chanf(`The results are in!`)
		winner := "nobody"
		for i := range votes {
			if i > 3 {
				break
			}

			player, votes := playerIndex[i], votes[i]
			acronym, index := players[player], i+1
			g.Chanf("%d. %s (%s, %d votes)", index, acronym, player, votes)
			if i == 0 {
				winner = player
			}
		}
		g.Chanf("Congratulations, %s!", commander.Bold(winner))
	}()
}

var games = map[string]*Game{}

var Acro = commander.Cmd("acro", func(s *commander.Source, r *commander.Response, cmd string, args []string) {
	usage := func() {
		r.Private()
		r.Printf("Usage: ACRO [#channel] {start|join|vote #|<submission>}")
	}
	originalArgs := args

	if len(args) == 0 {
		usage()
		return
	}

	var channel string
	if args[0][0] == '#' {
		channel = args[0]
		args = args[1:]
	} else {
		channel = s.Message().Args[0]
		if len(channel) == 0 || channel[0] != '#' {
			usage()
			return
		}
	}

	gamename := s.Server().Name() + "/" + channel
	var game *Game
	if g, ok := games[gamename]; ok {
		game = g
	} else {
		game = &Game{
			server:  s.Server(),
			channel: channel,
		}
		games[gamename] = game
	}

	if len(args) == 0 {
		usage()
		return
	}

	cmd, args = strings.ToLower(args[0]), args[1:]
	log.Printf("(%s) !acro %s %v", game, cmd, args)

	switch cmd {
	case "start":
		if game.started {
			r.Private()
			r.Printf("Acro has already been started in %s", channel)
			return
		}

		// Make sure the game keeps up with the server
		game.server = s.Server()

		// Start the game
		game.start()

	case "join":
		r.Private()
		if !game.started {
			r.Printf("You need to start the game before you can join it!")
			return
		}

		j := new(join)
		j.nick = s.ID().Nick
		j.ret = make(chan string)
		game.commands <- j
		r.Private()
		for msg := range j.ret {
			r.Printf(msg)
		}
	case "vote":
		if !game.started {
			r.Private()
			r.Printf("You can't vote right now.  Try starting a new game?")
			return
		}

		if len(args) < 1 {
			r.Private()
			r.Printf("Which acronym did you want to vote for?")
			return
		}

		idx, err := strconv.Atoi(args[0])
		if err != nil {
			r.Private()
			r.Printf("I don't recognize %s as a number: %s", args[0], err)
			return
		}

		v := new(vote)
		v.nick = s.ID().Nick
		v.idx = idx-1
		v.ret = make(chan string)
		game.commands <- v
		r.Private()
		for msg := range v.ret {
			r.Printf(msg)
		}
	default:
		args = originalArgs
		fallthrough
	case "submit":
		if !game.started {
			r.Private()
			r.Printf("You can't submit an acronym right now.  Try starting a new game?")
			return
		}

		sub := new(submission)
		sub.acro = strings.Join(args, " ")
		sub.nick = s.ID().Nick
		sub.ret = make(chan string)
		game.commands <- sub
		r.Private()
		for msg := range sub.ret {
			r.Printf(msg)
		}
	}


}).Args(1, -1).Help(`Play Acro!
Usage:
	ACRO [#channel] START
	ACRO [#channel] JOIN
	ACRO [#channel] SUBMIT <acronym>
	ACRO [#channel] VOTE <number>

	Acro is a game in which players complete to come up with the cleverest,
	funniest, weirdest, or most topical acronyms.  For instance, if the
	random acronym is:
		PAIMFP
	A valid acronym could be
		Playing ACRO is my favorite pastime!
`)

type gc struct {
	ret  chan string
}

func (g *gc) done() {
	close(g.ret)
}

func (g *gc) writef(format string, args ...interface{}) {
	g.ret <- fmt.Sprintf(format, args...)
}

type join struct {
	nick string
	gc
}

type submission struct {
	nick string
	acro string
	gc
}

type vote struct {
	nick string
	idx  int
	gc
}

type votesort struct {
	votes []int
	names []string
}

func (vs votesort) Len() int { return len(vs.votes) }
func (vs votesort) Less(i, j int) bool { return vs.votes[j] < vs.votes[i] }
func (vs votesort) Swap(i, j int)      { vs.votes[i], vs.votes[j], vs.names[i], vs.names[j] = vs.votes[j], vs.votes[i], vs.names[j], vs.names[i] }
