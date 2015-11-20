package bot

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/config"
	"github.com/enmand/quarid-go/pkg/irc"
	"github.com/enmand/quarid-go/pkg/logger"
	"github.com/enmand/quarid-go/pkg/plugin"
	"github.com/enmand/quarid-go/vm"
	"github.com/enmand/quarid-go/vm/js"

	"github.com/boltdb/bolt"
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

	return nil
}
func GetNick(userMask string) string {
	split := strings.SplitN(userMask, "@", 2)
	if len(split) > 0 {
		ident := strings.SplitN(split[0], "!", 2)
		if len(ident) > 0 {
			return ident[0]
		}
	}
	return ""
}

func (q *quarid) EnableSeen(db *bolt.DB) {
	var bucket *bolt.Bucket
	db.Batch(func(tx *bolt.Tx) error {
		bucket = tx.Bucket([]byte("seen"))
		return nil
	})
	seenLog := func(ev *adapter.Event, c adapter.Responder) {
		go func(ev *adapter.Event, c adapter.Responder, db *bolt.Bucket) {
			nick := strings.ToLower(GetNick(ev.Prefix))
			if len(nick) > 0 {
				if len(ev.Parameters[0]) > 0 &&
					ev.Parameters[0][0:1] == "#" {
					chanTime := fmt.Sprintf(
						"%v:%v",
						ev.Parameters[0],
						time.Now().Unix(),
					)
					if err := db.Put([]byte(nick), []byte(chanTime)); err != nil {
						logger.Log.Printf("Seen log error: %v\n", err.Error())
					}
				}
			}
		}(ev, c, bucket)
	}

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_PRIVMSG}},
		seenLog,
	)

}

func (q *quarid) LoadServices(services ...Service) {
	prvMsg := func(ev *adapter.Event, c adapter.Responder) {
		go func(ev *adapter.Event, c adapter.Responder, services ...Service) {
			for _, service := range services {
				if cmd, ok := readCommand(ev, service); ok {
					for _, command := range service.Commands() {
						if command.Name == cmd.Name {
							runCommand(cmd, command, c)
						}
					}
				}
			}
		}(ev, c, services...)
	}

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_PRIVMSG}},
		prvMsg,
	)

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
		q.operBot,
	)

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

func (q *quarid) operBot(
	ev *adapter.Event,
	c adapter.Responder,
) {

	ircOper := q.Config.GetString("irc.operator.user")
	ircPass := q.Config.GetString("irc.operator.pass")
	if len(ircOper) > 0 && len(ircPass) > 0 {
		logger.Log.Info("Logging in as Oper...")
		ev := &adapter.Event{
			Command:    irc.IRC_OPER,
			Parameters: []string{ircOper, ircPass},
		}
		q.IRC.Write(ev)
	} else {
		logger.Log.Info("No Oper settings detected")
	}

}
