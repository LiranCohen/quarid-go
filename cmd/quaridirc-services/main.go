package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/bot"
	"github.com/enmand/quarid-go/pkg/config"
	"github.com/enmand/quarid-go/pkg/database"
	"github.com/enmand/quarid-go/pkg/logger"

	"github.com/boltdb/bolt"
	"golang.org/x/crypto/bcrypt"
)

var DB *bolt.DB

func main() {
	c := config.Get()

	logger.Log.Info("Loading DB...")
	var err error
	DB, err = database.NewBolt("services")
	if err != nil {
		logger.Log.Panic(err)
	}

	if err := DB.Batch(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("nicks"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("hostmasks"))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte("chans"))
		return err
	}); err != nil {
		logger.Log.Panic(err)
	}
	logger.Log.Info("Loading IRC bot...")
	q := bot.New(&c)
	q.LoadServices(MakeNickBot(), MakeChanBot())

	if err := q.Connect(); err != nil {
		logger.Log.Errorf("%s", err)
		os.Exit(-1)
	}
	defer func() {
		q.Disconnect()
	}()
}

func MakeChanBot() bot.GenServ {

	chanBot := bot.NewService(
		"ChanBot",
		"Manage Channels for registered users",
		"!",
	)

	cmdOp := bot.NewCommand(
		"OP",
		"OP a user or yourself within a channel",
	)

	cmdOp.Parameters[0] = bot.CmdParam{
		Name:        "Nick",
		Description: []string{"Nick you would like to OP"},
		Required:    false,
	}

	cmdOp.Parameters[1] = bot.CmdParam{
		Name:        "Channel",
		Description: []string{"Channel to OP in"},
		Required:    false,
	}

	cmdOp.Handler = func(cmd bot.CmdOut, c adapter.Responder) {
		reqParams := 0
		for _, param := range cmdOp.Parameters {
			if param.Required {
				reqParams++
			}
		}
		if len(cmd.Params) < reqParams {
			//Display Usage message
			cmd.Respond(c, "Not enough params")
			return
		} else {
			nick := cmd.GetNick()
			if CheckSession(nick, cmd.UserMask) {
				if err := ChanPermission(nick, cmd.Channel); err == nil {
					cmd.ChanMode(c, "+o", nick)
				} else {
					cmd.Respond(c, "No Permissions")
				}
			} else {
				cmd.Respond(c, "Must login")
			}
		}
	}

	cmdAddOp := bot.NewCommand(
		"ADDOP",
		"Add a user to a channel's OP list",
	)

	cmdDropOp := bot.NewCommand(
		"DROPOP",
		"Drop a user from a channel's OP list",
	)

	cmdRegChan := bot.NewCommand(
		"REGCHAN",
		"Register a channel",
	)

	cmdRegChan.Channel = false

	cmdRegChan.Parameters[0] = bot.CmdParam{
		Name:        "Channel",
		Description: []string{"Channel you would like registered"},
		Required:    true,
	}
	cmdRegChan.Handler = func(cmd bot.CmdOut, c adapter.Responder) {
		reqParams := 0
		for _, param := range cmdRegChan.Parameters {
			if param.Required {
				reqParams++
			}
		}
		if len(cmd.Params) < reqParams {
			//Display Usage message
			cmd.Respond(c, "Not enough params")
			return
		} else {

			nick := cmd.GetNick()
			if CheckSession(nick, cmd.UserMask) {
				if err := ChanPermission(nick, cmd.Params[0]); err != nil {
					if err.Error() == "no permission" {
						cmd.Respond(c, "Channel already registered")
						return
					} else if err.Error() == "no chan" {
						//Register Channel
						RegisterChannel(nick, cmd.Params[0])
						cmd.Respond(c, "You are now the owner of "+cmd.Params[0])
						return
					}
				} else {
					cmd.Respond(c, "You alrady have permissions")
					return
				}
			} else {
				cmd.Respond(c, "Must login")
			}
		}
	}

	chanBot.AddCommands(
		cmdOp,
		cmdAddOp,
		cmdDropOp,
		cmdRegChan,
	)

	return chanBot
}

func MakeNickBot() bot.GenServ {
	nickBot := bot.NewService(
		"NickBot",
		"Manage persistant user registration for the server",
		"#",
	)

	cmdLogin := bot.NewCommand(
		"IDENTIFY",
		"Identify yourself",
	)

	cmdLogin.Parameters[0] = bot.CmdParam{
		Name:        "Password",
		Description: []string{"Password you would like to use"},
		Required:    true,
	}

	cmdLogin.Channel = false

	cmdLogin.Handler = func(cmd bot.CmdOut, c adapter.Responder) {
		reqParams := 0
		for _, param := range cmdLogin.Parameters {
			if param.Required {
				reqParams++
			}
		}
		if len(cmd.Params) < reqParams {
			//Display Usage message
			cmd.Respond(c, "Not enough params")
			return
		} else {
			nick := cmd.GetNick()
			//check if exsits
			var passHash []byte
			passErr := DB.View(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("nicks"))
				v := b.Get([]byte(nick))
				if v == nil {
					cmd.Respond(c, "User doesn't exist")
					return errors.New("doesn't exist")
				}
				passHash = v
				return nil
			})
			if passErr == nil {
				err := bcrypt.CompareHashAndPassword(passHash, []byte(cmd.Params[0]))
				if err != nil {
					cmd.Respond(c, "Incorrect password")
					return
				}
				if err := Login(nick, cmd.UserMask); err == nil {
					cmd.Respond(c, "Logged In")
					return
				} else {
					cmd.Respond(c, "Unknown Login Error")
				}
				return
			} else if passErr.Error() == "doesn't exist" {
				return
			}
		}
		cmd.Respond(c, "Unknown Error")
		return
	}

	cmdReg := bot.NewCommand(
		"REGISTER",
		"Register a new nick",
	)

	cmdReg.Parameters[0] = bot.CmdParam{
		Name:        "Password",
		Description: []string{"Password you would like to use"},
		Required:    true,
	}

	cmdReg.Channel = false

	cmdReg.Handler = func(cmd bot.CmdOut, c adapter.Responder) {
		reqParams := 0
		for _, param := range cmdReg.Parameters {
			if param.Required {
				reqParams++
			}
		}
		if len(cmd.Params) < reqParams {
			//Display Usage message
			cmd.Respond(c, "Not enough params")
			return
		} else {
			nick := cmd.GetNick()
			//check if exsits
			pass := DB.Batch(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("nicks"))
				v := b.Get([]byte(nick))
				if v == nil {
					return nil
				}
				return errors.New("user exists")
			})
			if pass == nil {
				reg := DB.Batch(func(tx *bolt.Tx) error {
					b := tx.Bucket([]byte("nicks"))
					if len(cmd.Params[0]) < 5 {
						cmd.Respond(c, "Password must be at least 5 chars")
						return errors.New("password lenght")
					}
					hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Params[0]), 10)
					if err != nil {
						return err
					}
					err = b.Put([]byte(nick), hash)
					return err
				})
				if reg == nil {
					if err := Login(nick, cmd.UserMask); err == nil {
						cmd.Respond(c, "Registered & LoggedIn")
					} else {
						cmd.Respond(c, "Registered, But error logging in.")
					}
					return
				}
			} else {
				cmd.Respond(c, "User already exists")
				return
			}
		}
		cmd.Respond(c, "Unknown Error")
		return
	}

	nickBot.AddCommands(cmdReg, cmdLogin)

	return nickBot
}

func Login(nick, hostmask string) error {
	masks := strings.Split(hostmask, "@")
	var storeMask string
	if len(masks) > 1 {
		storeMask = nick + ":" + strings.Join(masks[1:], "@")
	}
	err := DB.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("hostmasks"))
		timeNow := strconv.Itoa(int(time.Now().Unix()))
		err := b.Put([]byte(storeMask), []byte(timeNow))
		return err
	})
	return err
}

func CheckSession(nick, hostmask string) bool {
	masks := strings.Split(hostmask, "@")
	var storeMask string
	if len(masks) > 1 {
		storeMask = nick + ":" + strings.Join(masks[1:], "@")
	}
	err := DB.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("hostmasks"))
		v := b.Get([]byte(storeMask))
		if v == nil {
			return errors.New("doesn't exist")
		}
		t, err := strconv.Atoi(string(v))
		if err != nil {
			return errors.New("bad conversion")
		}
		lastSess := time.Unix(int64(t), 0)
		maxDur, err := time.ParseDuration("1h")
		if err != nil {
			return errors.New("session ended")
		}
		if lastSess.Add(maxDur).After(time.Now()) {
			timeNow := strconv.Itoa(int(time.Now().Unix()))
			err := b.Put([]byte(storeMask), []byte(timeNow))
			return err
		}
		return errors.New("unknown error")
	})
	if err != nil {
		return false
	} else {
		return true
	}

}

func ChanPermission(nick, channel string) error {
	err := DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("chans"))
		v := b.Get([]byte(channel))
		if v != nil {
			cb := tx.Bucket([]byte(v))
			cv := cb.Get([]byte(nick))
			if cv != nil {
				return nil
			} else {
				return errors.New("no permission")
			}
		}
		return errors.New("no chan")
	})
	return err
}

func RegisterChannel(nick, channel string) error {
	err := DB.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("chans"))
		chanHash, err := bcrypt.GenerateFromPassword([]byte(channel), 1)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(channel), chanHash); err != nil {
			return err
		}
		cb, err := tx.CreateBucket(chanHash)
		if err != nil {
			return err
		}
		if err := cb.Put([]byte(nick), []byte("owner")); err != nil {
			return err
		}
		return nil
	})
	return err
}
