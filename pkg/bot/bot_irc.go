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

const CMD_SYM = "!"

func FormatCommand(symbol, input string) ([]string, error) {
	//check for symbol
	if len(symbol) < 1 || len(symbol) > 5 {
		return []string{}, errors.New("invalid symbol")
	}
	sym, err := regexp.Compile("^" + symbol + "(.*)$")
	if err != nil {
		return []string{}, errors.New("regex: invalid symbol")
	}
	if match := sym.FindString(input); len(match) > 0 {
		return strings.Split(match[1:], " "), nil
	}

	return []string{}, errors.New("invalid command")
}

func (q *quarid) checkChanAdmin(ev *adapter.Event) bool {
	adMasks, _ := q.ChanAdmins[ev.Parameters[0]]
	if len(adMasks) > 0 {
		rReg := maskRegexp(adMasks)
		fMask, err := FormatMask(ev.Prefix)
		if err != nil {
			logger.Log.Error(err)
			return false
		}
		return rReg.MatchString(fMask.Host)
	}
	return false
}

func (q *quarid) checkChanPermissions(ev *adapter.Event) bool {
	opMasks, _ := q.ChanOps[ev.Parameters[0]]
	adMasks, _ := q.ChanAdmins[ev.Parameters[0]]
	allMasks := append(opMasks, adMasks...)
	if len(allMasks) > 0 {
		rReg := maskRegexp(allMasks)
		fMask, err := FormatMask(ev.Prefix)
		if err != nil {
			logger.Log.Error(err)
			return false
		}
		return rReg.MatchString(fMask.Host)
	}
	return false
}

func (q *quarid) cmdChanOP(ev *adapter.Event, c adapter.Responder) {
	cmd, err := FormatCommand(CMD_SYM, ev.Parameters[1])
	if err != nil {
		return
	}
	if strings.ToUpper(cmd[0]) == "OP" {
		if q.matchMask(ev.Prefix) || q.checkChanPermissions(ev) {
			if err := q.OPUser(
				ev.Parameters[0],
				getNick(ev.Prefix),
			); err != nil {
				logger.Log.Error(err)
			}
		} else {
			q.permissionsMessage(ev, c)
		}
	}
}

func (q *quarid) cmdAddAdmin(ev *adapter.Event, c adapter.Responder) {
	cmd, err := FormatCommand(CMD_SYM, ev.Parameters[1])
	if err != nil {
		return
	}
	if strings.ToUpper(cmd[0]) == "ADDADMIN" {
		if q.matchMask(ev.Prefix) || q.checkChanAdmin(ev) {
			fMask, _ := FormatMask(cmd[1])
			q.ChanAdmins[ev.Parameters[0]] = append(
				q.ChanAdmins[ev.Parameters[0]],
				fMask.Host,
			)
		} else {
			q.permissionsMessage(ev, c)
		}
	}
}

func (q *quarid) cmdDropAdmin(ev *adapter.Event, c adapter.Responder) {
	cmd, err := FormatCommand(CMD_SYM, ev.Parameters[1])
	if err != nil {
		return
	}
	if strings.ToUpper(cmd[0]) == "ADDADMIN" {
		if q.matchMask(ev.Prefix) || q.checkChanAdmin(ev) {
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
	}
}

func (q *quarid) cmdAddOP(ev *adapter.Event, c adapter.Responder) {
	cmd, err := FormatCommand(CMD_SYM, ev.Parameters[1])
	if err != nil {
		return
	}
	if strings.ToUpper(cmd[0]) == "ADDOP" {
		if q.matchMask(ev.Prefix) || q.checkChanAdmin(ev) {
			fMask, _ := FormatMask(cmd[1])
			q.ChanOps[ev.Parameters[0]] = append(
				q.ChanOps[ev.Parameters[0]],
				fMask.Host,
			)
		} else {
			q.permissionsMessage(ev, c)
		}
	}
}

func (q *quarid) cmdDropOp(ev *adapter.Event, c adapter.Responder) {
	cmd, err := FormatCommand(CMD_SYM, ev.Parameters[1])
	if err != nil {
		return
	}
	if strings.ToUpper(cmd[0]) == "DROPOP" {
		if q.matchMask(ev.Prefix) || q.checkChanAdmin(ev) {
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
	}
}

func (q *quarid) handleQuit(ev *adapter.Event, c adapter.Responder) {
	if len(ev.Parameters) > 0 && ev.Parameters[0] == "quit" {
		logger.Log.Warnf("Recieved Quit Command From Server")
		logger.Log.Warnf("Shutting Down Bot")
		q.Disconnect()
		return
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
	prvFuncs := []adapter.HandlerFunc{
		q.cmdChanOP,
		q.cmdAddOP,
		q.cmdDropOp,
		q.cmdAddAdmin,
		q.cmdDropAdmin,
	}
	for _, hF := range prvFuncs {
		q.IRC.Handle(
			[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_PRIVMSG}},
			hF,
		)
	}

	q.IRC.Handle(
		[]adapter.Filter{irc.CommandFilter{Command: irc.IRC_ERROR}},
		q.handleQuit,
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
