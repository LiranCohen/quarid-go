package irc

// IRC Events
//
// The events system will coordinate events between reading from the server,
// and any actions that should be handled for those events, based on a Filter.

import (
	"bufio"
	"net/textproto"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/enmand/quarid-go/pkg/adapter"
)

// Loop reads the data from the server, and handles events that happen
func (i *Client) Loop() {
	go i.read()

	for m := range i.events {
		go i.handleEvent(m)
	}
	<-i.dead
}

// Handle defines events that should be filtered to preform a handler function.
// Using "*" or "" for a filter, will cause all events to be passed to the
// HandlerFunc.
func (i *Client) Handle(fs []adapter.Filter, hf adapter.HandlerFunc) {
	h := &adapter.Handler{
		Filters: fs,
		Handler: hf,
	}

	i.handlers = append(i.handlers, h)
}

func (i *Client) handleEvent(ev *adapter.Event) {
	log.Infof("Handling event: %#v", ev)
	for _, h := range i.handlers {
		for _, f := range h.Filters {
			log.Debugf("\tChecking filter: %#v", f)
			if f.Match(ev) {
				log.Debug("\t\tFilter matched")
				h.Handler(ev, i)
			}
		}
	}
}

func (i *Client) read() {
	r := bufio.NewReader(i.conn)
	tp := textproto.NewReader(r)

	for {
		l, err := tp.ReadLine()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Fatal(err)
		}
		ws := strings.Split(l, " ")

		ev := &adapter.Event{}

		prefix := ws[0]
		if len(prefix) > 0 && prefix[0] == ':' {
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
