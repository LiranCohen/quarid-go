package bot

import (
	"regexp"
	"strings"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/irc"
	"github.com/enmand/quarid-go/pkg/logger"
	//"github.com/enmand/quarid-go/pkg/config"
)

type MsgService struct {
	Services map[string]Service
}

type Service interface {
	Name() string
	Description() string
	Commands() []Command
	Prefix() string
}

type Command struct {
	Name        string
	Description string
	Restricted  bool
	Channel     bool
	Parameters  map[int]CmdParam //int = order starting at 0
	Handler     MsgHandler
}

func NewCommand(name, description string) Command {
	c := Command{
		Name:        strings.ToUpper(name),
		Description: description,
		Restricted:  false,
		Channel:     true,
	}
	c.Parameters = make(map[int]CmdParam)
	return c
}

type CmdParam struct {
	Name        string
	Description []string
	Required    bool
}

type MsgHandler func(cmdOut, adapter.Responder)

func (ms *MsgService) initialize(services ...Service) adapter.HandlerFunc {
	return func(ev *adapter.Event, c adapter.Responder) {
		for _, service := range services {
			if cmd, ok := readCommand(ev, service); ok {
				for _, command := range service.Commands() {
					if command.Name == cmd.Name {
						runCommand(cmd, command, c)
					}
				}
			}
		}
	}
}

type cmdOut struct {
	Name     string
	Params   []string
	Channel  string
	UserMask string
}

func (c cmdOut) Respond(text string, r adapter.Responder) {
	response := adapter.Event{
		Command: irc.IRC_PRIVMSG,
	}

	nick := c.GetNick()

	if len(c.Channel) > 0 {
		response.Parameters = append(response.Parameters, c.Channel)
		text = nick + ": " + text
	} else {
		response.Parameters = append(response.Parameters, nick)
	}
	response.Parameters = append(response.Parameters, text)
	r.Write(&response)
}
func (c cmdOut) ActionTo(text string, target string, r adapter.Responder) {
	response := adapter.Event{
		Command: irc.IRC_PRIVMSG,
	}

	if len(c.Channel) > 0 {
		response.Parameters = append(response.Parameters, c.Channel)
		text = "\x01ACTION " + text + " " + target + "\x01"
	} else {
		return
	}
	response.Parameters = append(response.Parameters, text)
	r.Write(&response)
}

func (c cmdOut) Action(text string, r adapter.Responder) {
	response := adapter.Event{
		Command: irc.IRC_PRIVMSG,
	}

	nick := c.GetNick()

	if len(c.Channel) > 0 {
		response.Parameters = append(response.Parameters, c.Channel)
		text = "\x01ACTION " + text + " " + nick + "\x01"
	} else {
		return
	}
	response.Parameters = append(response.Parameters, text)
	r.Write(&response)
}

func (c cmdOut) GetNick() string {
	split := strings.Split(c.UserMask, "@")
	if len(split) > 0 {
		ident := strings.Split(split[0], "!")
		if len(ident) > 0 {
			return ident[0]
		}
	}
	return ""
}

func checkChannel(s string) bool {
	if len(s) > 0 && s[0:1] == "#" {
		return true
	}
	return false
}

func runCommand(cmd cmdOut, c Command, r adapter.Responder) {
	if !c.Channel && len(cmd.Channel) > 0 {
		return
	}
	if c.Handler != nil {
		c.Handler(cmd, r)
	}
}

func readCommand(ev *adapter.Event, se Service) (cmdOut, bool) {
	cmd := cmdOut{}
	if len(ev.Parameters) > 1 {
		params := strings.Split(ev.Parameters[1], " ")
		for i, param := range params {
			if param == "" {
				params = append(params[:i], params[i+1:]...)
			}
		}
		if checkChannel(ev.Parameters[0]) {
			prefix := se.Prefix()
			cmd.Channel = ev.Parameters[0]
			if len(params) > 0 && len(params[0]) > len(prefix) {
				cmdString := params[0][len(prefix):]
				prfReg, err := regexp.Compile("^" + prefix + "(.*)$")
				if err != nil {
					return cmd, false
				}
				if match := prfReg.FindString(cmdString); len(match) > 0 {
					cmd.Name = strings.ToUpper(match[len(prefix):])
				} else {
					return cmd, false
				}
				cmd.Params = params[1:]
				cmd.UserMask = ev.Prefix
				return cmd, true
			}
		} else {
			if len(params) > 0 {
				cmd.Channel = ""
				cmd.Name = strings.ToUpper(params[0][1:])
				cmd.Params = params[1:]
				cmd.UserMask = ev.Prefix
				return cmd, true
			}
		}
	}
	return cmd, false
}

type GenServ struct {
	name        string
	description string
	commands    []Command
	prefix      string
}

func MakeChanBot() GenServ {
	chanBot := NewService(
		"ChanBot",
		"Manage Channels for registered users",
		"!",
	)

	cmdCheese := NewCommand(
		"CHEESE",
		"To cheese someone",
	)
	cmdCheese.Handler = func(cmd cmdOut, c adapter.Responder) {
		cmd.ActionTo("Marries", "DrCheese", c)
	}

	cmdOp := NewCommand(
		"OP",
		"OP a user or yourself within a channel",
	)
	cmdOp.Parameters[0] = CmdParam{
		Name:        "Nick",
		Description: []string{"Nick you would like to OP"},
		Required:    false,
	}
	cmdOp.Parameters[1] = CmdParam{
		Name:        "Channel",
		Description: []string{"Channel to OP in"},
		Required:    false,
	}
	cmdOp.Handler = func(cmd cmdOut, c adapter.Responder) {
		logger.Log.Warnf("Cmd: %#v\n", cmd)
		cmd.Respond("Response", c)
		cmd.Action("Killed", c)
	}

	cmdAddOp := NewCommand(
		"ADDOP",
		"Add a user to a channel's OP list",
	)

	cmdDropOp := NewCommand(
		"DROPOP",
		"Drop a user from a channel's OP list",
	)

	chanBot.commands = []Command{
		cmdOp,
		cmdAddOp,
		cmdDropOp,
		cmdCheese,
	}

	return chanBot
}

func MakeNickBot() GenServ {
	nickBot := NewService(
		"NickBot",
		"Manage persistant user registration for the server",
		"#",
	)
	cmdRegNick := NewCommand(
		"REGISTER",
		"Register a new nick",
	)
	nickBot.commands = []Command{cmdRegNick}

	return nickBot
}

func NewService(name, description, prefix string) GenServ {
	g := GenServ{
		name:        name,
		description: description,
		prefix:      prefix,
	}
	return g
}

func (gs GenServ) Name() string {
	return gs.name
}

func (gs GenServ) Description() string {
	return gs.description
}

func (gs GenServ) Commands() []Command {
	return gs.commands
}

func (gs GenServ) Prefix() string {
	return gs.prefix
}
