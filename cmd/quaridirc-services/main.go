package main

import (
	"os"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/bot"
	"github.com/enmand/quarid-go/pkg/config"
	"github.com/enmand/quarid-go/pkg/database"
	"github.com/enmand/quarid-go/pkg/logger"

	"github.com/boltdb/bolt"
)

var DB bolt.DB

func main() {
	c := config.Get()

	logger.Log.Info("Loading DB...")
	var err error
	DB, err = bolt.NewBolt("services")
	if err != nil {
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
		cmd.Respond(c, "Response")
		cmd.Action(c, "Killed")
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
	cmdRegNick := bot.NewCommand(
		"REGISTER",
		"Register a new nick",
	)
	cmdRegNick.Parameters[0] = bot.CmdParam{
		Name:        "Nick",
		Description: []string{"Nick you would like to register"},
		Required:    true,
	}

	cmdRegNick.Parameters[1] = bot.CmdParam{
		Name:        "Password",
		Description: []string{"Password you would like to use"},
		Required:    true,
	}

	nickBot.AddCommands(cmdRegNick)

	return nickBot
}
