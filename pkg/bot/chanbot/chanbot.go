package chanbot

import (
	"github.com/enmand/quarid-go/pkg/bot"
	"github.com/enmand/quarid-go/pkg/config"
	"log"
)

type chanbot struct {
	//Quarid Instance
	Q *bot.quarid

	// List of Owners
	Owners map[string]string //cleartext password for now

	// List of Global Operators
	Opers map[string]struct{}

	//List of Channel Operators
	ChanOps map[string][]string // map[channel][]mask

	//List of Channel Administrators
	ChanAdmins map[string][]string // map[channel][]mask
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

func (q *Quarid) matchMask(mask string) bool {
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

func (q *Quarid) SendPrv(destination, message string) error {
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

func (q *Quarid) OPUser(channel, user string) error {
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

func (q *Quarid) permissionsMessage(ev *adapter.Event, c adapter.Responder) {
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

func (q *Quarid) checkChanAdmin(ev *adapter.Event) bool {
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

func (q *Quarid) checkChanPermissions(ev *adapter.Event) bool {
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

//Sets Mode +o to the given user
//Can Be in format [/msg <botname> #channel <username>]
//Or from within the Channel
func (q *Quarid) cmdChanOP(ev *adapter.Event, c adapter.Responder) {
	cmd, err := FormatCommand(CMD_SYM, ev.Parameters[1])
	if err != nil && !IsChannel(ev.Prefix) {

	} else if err != nil {
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

func (q *Quarid) cmdAddAdmin(ev *adapter.Event, c adapter.Responder) {
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

func (q *Quarid) cmdDropAdmin(ev *adapter.Event, c adapter.Responder) {
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

func (q *Quarid) cmdAddOP(ev *adapter.Event, c adapter.Responder) {
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

func (q *Quarid) cmdDropOp(ev *adapter.Event, c adapter.Responder) {
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
