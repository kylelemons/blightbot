package commander

import (
	"github.com/kylelemons/blightbot/bot"
	"sort"
	"strings"
)

type Hook func(cmd string, r *Response, args []string)

func (h Hook) call(cmd string, r *Response, args []string) {
	go func() {
		defer r.done()
		h(cmd, r, args)
	}()
}

type Command struct {
	name     string
	help     string
	hook     Hook
	min, max int
	priv     bool
}

// Args limits the command to only be called when the given minimum or
// maximum number of arguments are given.  If min or max is less than
// zero, that bound is effectively ignored.  The command is returned
// for easy chaining.
func (c *Command) Args(min, max int) *Command {
	if max < 0 {
		max = 32
	}
	c.min, c.max = min, max
	return c
}

// Private makes the command private, that is, it may only be called via
// private message and not in a channel.  The command is returned for
// easy chaining.
func (c *Command) Private() *Command {
	c.priv = true
	return c
}

// Help sets the help text for the command
func (c *Command) Help(text string) *Command {
	c.help = text
	return c
}

// Cmd creates a new Command out of the given hook using the given name.
func Cmd(name string, hook Hook) *Command {
	return &Command{
		name: name,
		hook: hook,
		min:  0,
		max:  32,
		priv: false,
	}
}

// Run creates the proper bindings on the bot and listens for commands on its
// servers.  This function does not exit, and so it should be called in its own
// goroutine if further work needs to be done.
func Run(b *bot.Bot, startchar byte, cmds []*Command) {
	// Handy local type for bundling data
	type event struct {
		name string
		srv  *bot.Server
		msg  *bot.Message
	}

	// Make the event handler
	events := make(chan event, 10)
	handle := func(evname string, srv *bot.Server, msg *bot.Message) {
		events <- event{evname, srv, msg}
	}

	// Listen for the events we want
	for _, evname := range []string{
		bot.ON_CHANMSG,
		bot.ON_PRIVMSG,
		bot.ON_NOTICE,
	}{
		b.OnEvent(evname, handle)
	}

	// Sort the commands for help
	sort.Sort(commandSorter(cmds))

	// Map the commands for easy access
	cmdmap := make(map[string][]*Command, len(cmds))
	cmdlen := 0
	for _, cmd := range cmds {
		cmd.name = strings.ToUpper(cmd.name)
		cmdmap[cmd.name] = append(cmdmap[cmd.name], cmd)
		if l := len(cmd.name); l > cmdlen {
			cmdlen = l
		}
	}

	// Add ping
	if _, ok := cmdmap["PING"]; !ok {
		c := &Command{
			name: "PING",
			help: "Built-in CTCP PING handler",
			priv: true,
			hook: func(cmd string, r *Response, args []string) {
				r.Private()
				r.Printf("PING %s", strings.Join(args, " "))
			},
		}
		cmdmap["PING"] = append(cmdmap["PING"], c)
		cmds = append(cmds, c)
	}

	// Add the help command
	if _, ok := cmdmap["HELP"]; !ok {
		c := &Command{
			name: "HELP",
			help: "Online help",
			priv: false,
		}
		cmdmap["HELP"] = append(cmdmap["HELP"], c)
		cmds = append(cmds, c)
		c.hook = genhelp(cmds, cmdlen)
	}

	// Wait for events and handle them
	for e := range events {
		// Ignore malformatted messages
		if len(e.msg.Args) < 2 || len(e.msg.Args[1]) == 0 {
			continue
		}

		// Determine if it is a command (CTCP or with the leader char)
		text, ctcp := e.msg.Args[1], false
		switch text[0] {
		case 0x01:
			text, ctcp = DecodeCTCP(text), true
		case startchar:
			text = text[1:]
		default:
			continue
		}

		// Parse the command into arguments
		command, args := "", strings.Fields(text)
		command, args = args[0], args[1:]

		// Look up the command
		cmd, ok := cmdmap[strings.ToUpper(command)]
		if !ok {
			continue
		}

		// Build the reply
		replies := make(chan *bot.Message, 10)
		go func() {
			if ctcp {
				for m := range replies {
					switch m.Command {
					case bot.CMD_PRIVMSG:
						fallthrough
					case bot.CMD_NOTICE:
						if len(m.Args) > 1 {
							m.Args[1] = EncodeCTCP(m.Args[1])
						}
					}
					e.srv.WriteMessage(m)
				}
				return
			}
			for m := range replies {
				e.srv.WriteMessage(m)
			}
		}()
		resp := &Response{
			out: replies,
		}

		// Set the public/private responses
		nick := e.msg.ID().Nick
		switch e.name {
		case bot.ON_CHANMSG:
			resp.public = e.msg.Args[0]
			resp.private = nick
		case bot.ON_PRIVMSG:
			resp.public = nick
			resp.private = nick
		case bot.ON_NOTICE:
			resp.public = ""
			resp.private = ""
		}

		// Call the hook
		for _, cmd := range cmd {
			go cmd.hook.call(command, resp, args)
		}
	}
}

func genhelp(cmds []*Command, cmdwidth int) func(cmd string, r *Response, args []string) {
	return func(cmd string, r *Response, args []string) {
		r.Private()
		r.Printf("Help:")

		name, sent := "", 0
		if len(args) > 0 {
			name = strings.ToUpper(args[0])
		}
		for _, cmd := range cmds {
			lines := strings.Split(cmd.help, "\n")
			if name != "" {
				if cmd.name == name {
					for _, line := range lines {
						r.Printf(line)
						sent++
					}
				}
				continue
			}
			r.Printf("  %*s - %s", cmdwidth, Bold(cmd.name), lines[0])
			sent++
		}
		if sent == 0 {
			r.Printf("  No matching commands found")
		}
	}
}

type commandSorter []*Command
func (c commandSorter) Len()          int  { return len(c) }
func (c commandSorter) Less(i, j int) bool { return c[i].name < c[j].name }
func (c commandSorter) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
