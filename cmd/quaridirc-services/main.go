package main

import (
	"errors"
	"os"

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

	cmdCheese := bot.NewCommand(
		"CHEESE",
		"To cheese someone",
	)

	cmdCheese.Handler = func(cmd bot.CmdOut, c adapter.Responder) {
		cmd.Action(c, "Marries DrCheese")
	}

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
		logger.Log.Warnf("Cmd: %#v\n", cmd)
		nick := cmd.GetNick()
		cmd.ChanMode(c, "+o", nick)
	}

	cmdAddOp := bot.NewCommand(
		"ADDOP",
		"Add a user to a channel's OP list",
	)

	cmdDropOp := bot.NewCommand(
		"DROPOP",
		"Drop a user from a channel's OP list",
	)

	chanBot.AddCommands(
		cmdOp,
		cmdAddOp,
		cmdDropOp,
		cmdCheese,
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
				cmd.Respond(c, "Logged in")
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
				logger.Log.Printf("Reg: %#v\n", reg)
				if reg == nil {
					cmd.Respond(c, "Registered")
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
