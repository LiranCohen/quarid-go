package bot

import (
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
}

type Command struct {
	Name        string
	Description string
	Restricted  bool
	Channel     bool
	Parameters  map[int]CmdParam //int = order starting at 0
	Handler     MsgHandler
}

func NewCommand(name, description string) *Command {
	c := Command{
		Name:        name,
		Description: description,
		Restricted:  false,
		Channel:     true,
	}
	return &c
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

type MsgHandler func(*Cmd, adapter.Responder)

func (ms *MsgService) initialize(services ...Service) adapter.HandlerFunc {
	return func(ev *adapter.Event, c adapter.Responder) {}
}

type ChanBot struct {
	Name        string
	Description string
	Commands    []Command
}

func (cb *ChanBot) Name() string {
	return cb.Name
}

func (cb *ChanBot) Description() string {
	return cb.Description
}

func (cb *ChanBot) Commands() (commands []Command) {
	return cb.Commands
	cmdOp := Command{
		Name:        "OP",
		Description: "OP yourself or another user within a channel",
		Parameters: map[string][]string{
			"NICK": {
				"(optional) The Nick of the use you would like to OP, leave blank to OP yourself",
			},
			"CHANNEL": {
				"(optional) If sent in a Private Message please include the target channel",
			},
		},
	}
	checkChannel = func(s string) bool {
		if len(s) > 0 && s[0:1] == "#" {
			return true
		}
		return false
	}
	cmdOp.Handler = func(*Cmd, adapter.Responder) {
		if Cmd.Name == "OP" {
			switch len(Cmd.Parameters) {
			case 0:
				if !checkChannel(Cmd.Parameters[0]) {
				}
			case 1:
			case 2:
			default:
			}
		}
	}
	return
}

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
