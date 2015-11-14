package bot

import (
	"regexp"
	"strings"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/logger"
	//"github.com/enmand/quarid-go/pkg/config"
)

type MsgService struct {
	Services map[string]MsgHandler
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
		Name:        name,
		Description: description,
		Restricted:  false,
		Channel:     true,
	}
	return c
}

type CmdParam struct {
	Name        string
	Description []string
	Required    bool
}

type PrvMsg struct {
	Prefix  string
	Source  string
	Message string
}

type MsgHandler func(*cmdOut, adapter.Responder)

func (ms *MsgService) initialize(services ...Service) adapter.HandlerFunc {
	return func(ev *adapter.Event, c adapter.Responder) {
		for _, service := range services {
			if cmd, ok := readCommand(ev, service); ok {
				logger.Log.Printf("%#v", cmd)
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

func checkChannel(s string) bool {
	if len(s) > 0 && s[0:1] == "#" {
		return true
	}
	return false
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
				cmd.Name = strings.ToUpper(params[0])
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

	cmdOp := NewCommand(
		"OP",
		"OP a user or yourself within a channel",
	)

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
	}

	return chanBot
}

func MakeNickBot() GenServ {
	nickBot := NewService(
		"NickBot",
		"Manage persistant user registration for the server",
		"#",
	)

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

//func (cb *ChanBot) Commands() (commands []Command) {
//return cb.Commands
//cmdOp := Command{
//Name:        "OP",
//Description: "OP yourself or another user within a channel",
//Parameters: map[string][]string{
//"NICK": {
//"(optional) The Nick of the use you would like to OP, leave blank to OP yourself",
//},
//"CHANNEL": {
//"(optional) If sent in a Private Message please include the target channel",
//},
//},
//}
//cmdOp.Handler = func(*Cmd, adapter.Responder) {
//if Cmd.Name == "OP" {
//switch len(Cmd.Parameters) {
//case 0:
//if !checkChannel(Cmd.Parameters[0]) {
//}
//case 1:
//case 2:
//default:
//}
//}
//}
//return
//}

//func (pm *PrvMsg) cmdChanOP() {

//}

//func (pm *PrvMsg) cmdAddOp() {

//}

//func (pm *PrvMsg) cmdDropOp() {

//}

//func (pm *PrvMsg) cmdAddAdmin() {

//}

//func (pm *PrvMsg) cmdAddAdmin() {

//}
