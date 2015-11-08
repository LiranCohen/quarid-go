package bot

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/config"
	"github.com/enmand/quarid-go/pkg/irc"
	"github.com/enmand/quarid-go/pkg/logger"
	"github.com/enmand/quarid-go/pkg/plugin"
	"github.com/enmand/quarid-go/vm"
	"github.com/enmand/quarid-go/vm/js"
)

type quarid struct {
	// Connection to the IRC server
	IRC *irc.Client

	// Configuration from the user
	Config *config.Config

	// List of Opers
	Opers map[string]struct{} //list of masks

	// The Plugins we have loaded
	plugins []plugin.Plugin

	// The VM for our Plugins
	vms map[string]vm.VM
}

func (q *quarid) initialize() error {
	q.IRC = irc.NewClient(
		q.Config.GetString("irc.nick"),
		q.Config.GetString("irc.user"),
		q.Config.GetBool("irc.tls.verify"),
		q.Config.GetBool("irc.tls.enable"),
	)

	user := q.Config.GetString("irc.user")
	password := q.Config.GetString("irc.password")
	if len(user) > 0 && len(password) > 0 {
		q.IRC.OPerUser = user
		q.IRC.OPerPass = password
	}

	// Initialize our VMs
	q.vms = map[string]vm.VM{
		vm.JS: js.NewVM(),
	}
	q.Opers = make(map[string]struct{})
	admins := q.Config.GetStringSlice("irc.admins")
	for _, admin := range admins {
		q.Opers[admin] = struct{}{}
	}

	var errs []error
	q.plugins, errs = q.LoadPlugins(q.Config.GetStringSlice("plugins_dirs"))
	if errs != nil {
		logger.Log.Warningf(
			"Some plugins failed to load. The following are loaded: %q",
			q.plugins,
		)
		logger.Log.Warningf("But the follow errors occurred:")
		for _, e := range errs {
			logger.Log.Warning(e)
		}
	}

	return nil
}

func (q *quarid) LoadPlugins(dirs []string) ([]plugin.Plugin, []error) {
	var ps []plugin.Plugin
	var errs []error

	for _, d := range dirs {
		fis, err := ioutil.ReadDir(d)
		if err != nil {
			errs = append(errs, err)
		}

		for _, fi := range fis {
			if fi.IsDir() {
				p := plugin.NewPlugin(
					fi.Name(),
					fmt.Sprintf("%s/%s", d, fi.Name()),
				)
				if err := p.Load(q.VMs()); err != nil {
					errs = append(errs, err)
				} else {
					ps = append(ps, p)
				}

			}
		}
	}

	return ps, errs
}

func getNick(mask string) string {
	split := strings.Split(mask, "@")
	if len(split) > 0 {
		ident := strings.Split(mask, "!")
		if len(ident) > 0 {
			return ident[0]
		}
	}
	return ""
}

func (q *quarid) matchMask(mask string) bool {
	return true
}

func (q *quarid) SendPrv(destination, message string) error {
	response := adapter.Event{
		Command: irc.IRC_PRIVMSG,
		Parameters: []string{
			destination,
			message,
		},
	}
	if err := q.IRC.Write(&response); err != nil {
		return err
	}
	return nil
}

func (q *quarid) OPUser(channel, user string) error {
	response := adapter.Event{
		Command: irc.IRC_MODE,
		Parameters: []string{
			channel,
			"+o",
			user,
		},
	}
	if err := q.IRC.Write(&response); err != nil {
		return err
	}
	return nil
}

func (q *quarid) prvMsg(ev *adapter.Event, c adapter.Responder) {
	logger.Log.Printf("Handle Private Message: %v\n", ev)
	if len(ev.Parameters) > 1 {
		if ev.Parameters[0] == q.IRC.Nick {
			return
		}
		bang := regexp.MustCompile("^!(.*)$")
		if match := bang.FindString(ev.Parameters[1]); len(match) > 0 {
			cmd := strings.Split(match[1:], " ")
			if len(cmd) > 0 {
				switch strings.ToUpper(cmd[0]) {
				case "OP":
					if q.matchMask(ev.Prefix) {
						if err := q.OPUser(
							ev.Parameters[0],
							getNick(ev.Prefix),
						); err != nil {
							logger.Log.Error(err)
						}
					} else {
						message := fmt.Sprintf("%v: Go Fuck Yourself!")
						if err := q.SendPrv(ev.Parameters[0], message); err != nil {
							logger.Log.Error(err)
						}
					}
				default:
					message := fmt.Sprintf("%v: Unknown command", getNick(ev.Prefix))
					if err := q.SendPrv(ev.Parameters[0], message); err != nil {
						logger.Log.Error(err)
					}
				}
			}
		}
	}
}

func (q *quarid) Connect() error {
	err := q.IRC.Connect(q.Config.GetString("irc.server"))
	if err != nil {
		return err
	}

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_RPL_MYINFO}},
		q.joinChan,
	)

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_PRIVMSG}},
		q.prvMsg,
	)

	q.IRC.Loop()

	return err
}

func (q *quarid) Disconnect() {
	q.IRC.Disconnect()
}

func (q *quarid) Plugins() []plugin.Plugin {
	return q.plugins
}

func (q *quarid) VMs() map[string]vm.VM {
	return q.vms
}

func (q *quarid) joinChan(
	ev *adapter.Event,
	c adapter.Responder,
) {
	if len(q.IRC.OPerUser) > 0 {
		if err := c.Write(&adapter.Event{
			Command: irc.IRC_OPER,
			Parameters: []string{
				q.IRC.OPerUser,
				q.IRC.OPerPass,
			},
		}); err != nil {
			logger.Log.Error(err)
		}
	}

	chans := q.Config.GetStringSlice("irc.channels")

	joinCmd := &adapter.Event{
		Command:    irc.IRC_JOIN,
		Parameters: chans,
	}
	c.Write(joinCmd)
}
