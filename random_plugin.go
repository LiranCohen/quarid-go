package main

import (
	"math/rand"
	"regexp"
	"time"

	"github.com/thoj/go-ircevent"
)

func init() {
	const PLUGIN_NAME = "random"

	trigger := regexp.MustCompile("^@random$")

	commands := TriggerCommands{
		trigger: Command{
			Handler: randomHandler,
		},
	}

	plugin := Plugin{
		Commands: commands,
	}

	RegisterPlugin(PLUGIN_NAME, plugin)
}

func randomHandler(con *irc.Connection, config *Config, msgCh chan *Message) {
	msg := <-msgCh
	rand.Seed(time.Now().Unix())
	rand := rand.ExpFloat64()
	con.Privmsgf(msg.Room, "%f", rand)
}
