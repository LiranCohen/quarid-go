package bot

import (
	"fmt"

	"github.com/enmand/quarid-go/pkg/config"
	"github.com/enmand/quarid-go/pkg/plugin"
	"github.com/enmand/quarid-go/vm"
)

// Bot can connect to a service and forward events
type Bot interface {
	// Load all of the plugins in each `dir`
	LoadPlugins(dir []string) ([]plugin.Plugin, []error)

	// Connect to the configured server
	Connect() error

	// Disconnect from the server
	Disconnect()

	// Return a map of initialzed Plugins for this bot
	Plugins() []plugin.Plugin

	// A list of VMs that the bot has available
	VMs() map[string]vm.VM
}

// New returns a new instance of a Bot
func New(cfg *config.Config) *Quarid {
	bot := &Quarid{
		Config: cfg,
	}

	err := bot.initialize()
	if err != nil {
		panic(fmt.Sprintf("Could not initialize bot: %s", err))
	}

	return bot
}
