package bot

import (
	"regexp"
	"strings"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/irc"
	//"github.com/enmand/quarid-go/pkg/logger"
	//"github.com/enmand/quarid-go/pkg/config"
)

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

type MsgHandler func(CmdOut, adapter.Responder)

type CmdOut struct {
	Name     string
	Params   []string
	Channel  string
	UserMask string
}

//Send response to user who issued the command
//If user sent private message, response is in private message
//If user sent a channel command, response is in the channel
func (c CmdOut) Respond(r adapter.Responder, text string) {
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

//Send an action to the channel which the command was issued
//No action is sent if command was not issued from a channel
func (c CmdOut) Action(r adapter.Responder, text string) {
	response := adapter.Event{
		Command: irc.IRC_PRIVMSG,
	}

	if len(c.Channel) > 0 {
		response.Parameters = append(response.Parameters, c.Channel)
		text = "\x01ACTION " + text + "\x01"
	} else {
		return
	}
	response.Parameters = append(response.Parameters, text)
	r.Write(&response)
}

//Send a general message into the channel which the command was issued
//No message is sent if command was not issued from a channel
func (c CmdOut) Message(r adapter.Responder, text string) {
	response := adapter.Event{
		Command: irc.IRC_PRIVMSG,
	}

	if len(c.Channel) > 0 {
		response.Parameters = append(response.Parameters, c.Channel)
	} else {
		return
	}

	response.Parameters = append(response.Parameters, text)
	r.Write(&response)
}

//Send a MODE command to the channel which the command was issued
//No mode is sent if command was not issued from a channel
func (c CmdOut) ChanMode(r adapter.Responder, params ...string) {
	response := adapter.Event{
		Command: irc.IRC_MODE,
	}

	if len(c.Channel) > 0 {
		response.Parameters = append(response.Parameters, c.Channel)
	} else {
		return
	}

	response.Parameters = append(response.Parameters, params...)
	r.Write(&response)
}

//Retrieve the Nick of the user who issued the command
//Gets the nick from the user's hostmask
func (c CmdOut) GetNick() string {
	split := strings.SplitN(c.UserMask, "@", 2)
	if len(split) > 0 {
		ident := strings.SplitN(split[0], "!", 2)
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

func runCommand(cmd CmdOut, c Command, r adapter.Responder) {
	if !c.Channel && len(cmd.Channel) > 0 {
		return
	}
	if c.Handler != nil {
		c.Handler(cmd, r)
	}
}

func readCommand(ev *adapter.Event, se Service) (CmdOut, bool) {
	cmd := CmdOut{}
	if len(ev.Parameters) > 1 {
		allparams := strings.Split(ev.Parameters[1], " ")
		params := []string{}
		for _, param := range allparams {
			if param != "" {
				params = append(params, param)
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

func NewService(name, description, prefix string) GenServ {
	g := GenServ{
		name:        name,
		description: description,
		prefix:      prefix,
	}
	return g
}

func (gs *GenServ) AddCommands(commands ...Command) {
	for _, command := range commands {
		gs.commands = append(gs.commands, command)
	}
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
