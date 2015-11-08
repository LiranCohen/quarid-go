package bot

import (
	"errors"
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
	Opers map[string]struct{}

	//List of Channel Ops
	ChanOps map[string][]string // map[channel][]mask

	//List of Channel Ops
	ChanAdmins map[string][]string // map[channel][]mask

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
	q.ChanOps = make(map[string][]string)
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

type HostMask struct {
	Nick string
	User string
	Host string
}

func FormatMask(mask string) (HostMask, error) {
	hm := HostMask{}
	hostSplit := strings.Split(mask, "@")
	if len(hostSplit) > 1 {
		preSplit := strings.Split(hostSplit[0], "!")
		hm.Host = hostSplit[1]
		if len(preSplit) > 1 {
			hm.Nick = preSplit[0]
			hm.User = preSplit[1]
			return hm, nil
		} else if len(preSplit) > 0 && preSplit[0] == "*" {
			hm.Nick = "*"
			hm.User = "*"
			return hm, nil
		}
	}
	return hm, errors.New("invalid mask")
}

//Taken from github.com/edmund-huber/ergonomadic
// Generate a regular expression from the set of user mask
// strings. Masks are split at the two types of wildcards, `*` and
// `?`. All the pieces are meta-escaped. `*` is replaced with `.*`,
// the regexp equivalent. Likewise, `?` is replaced with `.`. The
// parts are re-joined and finally all masks are joined into a big
// or-expression.
func maskRegexp(masks []string) *regexp.Regexp {
	rReg := &regexp.Regexp{}
	if len(masks) == 0 {
		rReg = nil
		return rReg
	}

	maskExprs := make([]string, len(masks))
	for index, mask := range masks {
		manyParts := strings.Split(mask, "*")
		manyExprs := make([]string, len(manyParts))
		for mindex, manyPart := range manyParts {
			oneParts := strings.Split(manyPart, "?")
			oneExprs := make([]string, len(oneParts))
			for oindex, onePart := range oneParts {
				oneExprs[oindex] = regexp.QuoteMeta(onePart)
			}
			manyExprs[mindex] = strings.Join(oneExprs, ".")
		}
		maskExprs[index] = strings.Join(manyExprs, ".*")
	}
	expr := "^" + strings.Join(maskExprs, "|") + "$"
	rReg, _ = regexp.Compile(expr)
	return rReg
}

func (q *quarid) matchMask(mask string) bool {
	fMask, err := FormatMask(mask)
	if err != nil {
		logger.Log.Error(err)
	}
	aMasks := []string{}
	for oper, _ := range q.Opers {
		operMask, err := FormatMask(oper)
		if err != nil {
			logger.Log.Error(err)
			continue
		}
		if operMask.Nick == "*" || operMask.Nick == fMask.Nick {
			if operMask.User == "*" || operMask.User == fMask.User {
				aMasks = append(aMasks, operMask.Host)
			}
		}
	}
	rReg := maskRegexp(aMasks)
	if rReg.MatchString(fMask.Host) {
		return true
	}
	return false
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

func (q *quarid) permissionsMessage(ev *adapter.Event, c adapter.Responder) {
	message := fmt.Sprintf("%v: bugger off!", getNick(ev.Prefix))
	if err := q.SendPrv(ev.Parameters[0], message); err != nil {
		logger.Log.Error(err)
	}
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
					opMasks, _ := q.ChanOps[ev.Parameters[0]]
					if q.matchMask(ev.Prefix) {
						if err := q.OPUser(
							ev.Parameters[0],
							getNick(ev.Prefix),
						); err != nil {
							logger.Log.Error(err)
						}
					} else if len(opMasks) > 0 {
						rReg := maskRegexp(opMasks)
						fMask, err := FormatMask(ev.Prefix)
						if err != nil {
							logger.Log.Error(err)
						}
						if rReg.MatchString(fMask.Host) {
							if err := q.OPUser(
								ev.Parameters[0],
								getNick(ev.Prefix),
							); err != nil {
								logger.Log.Error(err)
							}
						} else {
							logger.Log.Printf("REGEX: %#v", rReg)
							q.permissionsMessage(ev, c)
						}
					} else {
						q.permissionsMessage(ev, c)
					}
				case "ADDADMIN":
					if q.matchMask(ev.Prefix) {
						fMask, _ := FormatMask(cmd[1])
						q.ChanAdmins[ev.Parameters[0]] = append(
							q.ChanAdmins[ev.Parameters[0]],
							fMask.Host,
						)
					} else {
						q.permissionsMessage(ev, c)
					}
				case "ADDOP":
					if q.matchMask(ev.Prefix) {
						fMask, _ := FormatMask(cmd[1])
						q.ChanOps[ev.Parameters[0]] = append(
							q.ChanOps[ev.Parameters[0]],
							fMask.Host,
						)
					} else {
						q.permissionsMessage(ev, c)
					}
				case "DROPOP":
					if q.matchMask(ev.Prefix) {
						fMask, _ := FormatMask(cmd[1])
						for i, mask := range q.ChanOps[ev.Parameters[0]] {
							if mask == fMask.Host {
								q.ChanOps[ev.Parameters[0]] = append(
									q.ChanOps[ev.Parameters[0]][:i],
									q.ChanOps[ev.Parameters[0]][i+1:]...,
								)

							}
						}
					} else {
						q.permissionsMessage(ev, c)
					}
				case "DROPADMIN":
					if q.matchMask(ev.Prefix) {
						fMask, _ := FormatMask(cmd[1])
						for i, mask := range q.ChanAdmins[ev.Parameters[0]] {
							if mask == fMask.Host {
								q.ChanAdmins[ev.Parameters[0]] = append(
									q.ChanAdmins[ev.Parameters[0]][:i],
									q.ChanAdmins[ev.Parameters[0]][i+1:]...,
								)

							}
						}
					} else {
						q.permissionsMessage(ev, c)
					}

				default:
					logger.Log.Printf("\n\n\n DEFAULT:%v\n\n\n", cmd[0])
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
