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

type quarid struct {
	// Connection to the IRC server
	IRC *irc.Client

	// Configuration from the user
	Config *config.Config

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

	services := []string{
		"chanServ",
		"nivkServ",
	}

	msgService := &MsgService{}
	prvMsg := msgService.initialize(services)

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_PRIVMSG}},
		prvMsg,
	)

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

func (q *quarid) Connect() error {
	go q.IRC.Loop()

	err := q.IRC.Connect(q.Config.GetString("irc.server"))
	if err != nil {
		return err
	}
	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_RPL_MYINFO}},
		q.joinChan,
	)

	rCh := make(chan error)
	go func(ch chan error) {
		ch <- q.IRC.Read()
	}(rCh)

	if readErr := <-rCh; readErr != nil {
		logger.Log.Errorf(err.Error())
		return err
	}

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
	chans := q.Config.GetStringSlice("irc.channels")

	joinCmd := &adapter.Event{
		Command:    irc.IRC_JOIN,
		Parameters: chans,
	}
	c.Write(joinCmd)
}
