package irc

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/textproto"
	"strings"
	"time"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/logger"
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

	logger.Log.Infof("Connected to %s", server)

	go i.authenticate()

	return err
}

// Disconnect disconnects this client from the server it's connected to
func (i *Client) Disconnect() error {
	err := i.Write(&adapter.Event{
		Command: IRC_QUIT,
	})

	i.dead <- false
	close(i.dead)
	close(i.events)

	return err
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

	err = i.Write(&adapter.Event{
		Command: IRC_USER,
		Parameters: []string{
			i.Ident,
			"0.0.0.0",
			"0.0.0.0",
			i.Ident,
			i.Nick,
		},
	})

	if err != nil {
		i.Disconnect()
	}
}

func (i *Client) read() {
	r := bufio.NewReader(i.conn)
	tp := textproto.NewReader(r)

	for {
		l, _ := tp.ReadLine()
		ws := strings.Split(l, " ")

		ev := &adapter.Event{}

		if prefix := ws[0]; prefix[0] == ':' {
			ev.Prefix = prefix[1:]
		} else {
			ev.Prefix = ""
			ev.Command = prefix
		}

		trailingIndex := 1
		if ev.Prefix != "" {
			trailingIndex = 2
			ev.Command = ws[1]
		}

		var trailing []string
		for _, param := range ws[trailingIndex:len(ws)] {
			if len(param) > 0 && (param[0] == ':' || len(trailing) > 0) {
				if param[0] == ':' {
					param = param[1:]
				}
				trailing = append(trailing, param)
			} else if len(trailing) == 0 {
				ev.Parameters = append(ev.Parameters, param)
			}
		}

		ev.Parameters = append(ev.Parameters, strings.Join(trailing, " "))
		ev.Timestamp = time.Now()

		i.events <- ev

		if ev.Command == IRC_PING {
			ev.Command = IRC_PONG
			go i.Write(ev)
		}
	}
}
