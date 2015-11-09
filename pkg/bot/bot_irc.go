package bot

import (
	"fmt"
	"io/ioutil"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/config"
	"github.com/enmand/quarid-go/pkg/irc"
	"github.com/enmand/quarid-go/pkg/logger"
	"github.com/enmand/quarid-go/pkg/plugin"
	"github.com/enmand/quarid-go/vm"
	"github.com/enmand/quarid-go/vm/js"
)

type Quarid struct {
	// Connection to the IRC server
	IRC *irc.Client

	// Configuration from the user
	Config *config.Config

	// The Plugins we have loaded
	plugins []plugin.Plugin

	// The VM for our Plugins
	vms map[string]vm.VM
}

func (q *Quarid) initialize() error {
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

func (q *Quarid) LoadPlugins(dirs []string) ([]plugin.Plugin, []error) {
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

func (q *Quarid) handleQuit(ev *adapter.Event, c adapter.Responder) {
	if len(ev.Parameters) > 0 && ev.Parameters[0] == "quit" {
		logger.Log.Warnf("Recieved Quit Command From Server")
		logger.Log.Warnf("Shutting Down Bot")
		q.Disconnect()
		return
	}
}

func (q *Quarid) Connect() error {
	err := q.IRC.Connect(q.Config.GetString("irc.server"))
	if err != nil {
		return err
	}

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_RPL_MYINFO}},
		q.joinChan,
	)

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_ERROR}},
		q.handleQuit,
	)
	q.IRC.Loop()

	return err
}

func (q *Quarid) Disconnect() {
	q.IRC.Disconnect()
}

func (q *Quarid) Plugins() []plugin.Plugin {
	return q.plugins
}

func (q *Quarid) VMs() map[string]vm.VM {
	return q.vms
}

func (q *Quarid) joinChan(
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
