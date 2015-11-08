package irc

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/logger"

	"github.com/renstrom/shortuuid"
)

// Connect connects this client to the server given
func (i *Client) Connect(server string) error {
	var err error

	i.events = make(chan *adapter.Event)
	i.dead = make(chan bool)

	if !i.TLS {
		i.conn, err = net.DialTimeout("tcp", server, TIMEOUT)
	} else {
		i.conn, err = tls.DialWithDialer(&net.Dialer{
			Timeout: TIMEOUT,
		}, "tcp", server, &tls.Config{
			InsecureSkipVerify: i.TLSVerify,
		})
	}
	if err != nil {
		return fmt.Errorf("Could not connect to server: %s", err)
	}

	logger.Log.Infof("Connecting to %s", server)

	go i.authenticate()

	return err
}

// Disconnect disconnects this client from the server it's connected to
func (i *Client) Disconnect() error {
	var err error

	err = i.Write(&adapter.Event{
		Command: IRC_QUIT,
	})

	err = i.conn.Close()

	i.dead <- false
	close(i.dead)
	close(i.events)

	return err
}
func (i *Client) fixNick(
	ev *adapter.Event,
	c adapter.Responder,
) {
	nick := i.Nick
	uniq := shortuuid.UUID()

	newNick := fmt.Sprintf("%s_%s", nick, uniq)
	i.Nick = newNick
	i.authenticate()

}

func (i *Client) authenticate() {
	var err error
	logger.Log.Infof("Authenticating for nick %s!%s", i.Nick, i.Ident)
	err = i.Write(&adapter.Event{
		Command: IRC_NICK,
		Parameters: []string{
			i.Nick,
		},
	})

	i.Handle(
		[]adapter.Filter{CommandFilter{Command: IRC_ERR_NICKNAMEINUSE}},
		i.fixNick,
	)

	// RFC 2812 USER command
	err = i.Write(&adapter.Event{
		Command: IRC_USER,
		Parameters: []string{
			i.Ident,
			"0",
			"*",
			i.Nick,
		},
	})

	if err != nil {
		i.Disconnect()
	}
}
