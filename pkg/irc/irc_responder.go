package irc

// Responder
//
// Responder responds to the IRC server, by writing an IRC Event to the CLient's
// connection

import (
	"bytes"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/enmand/quarid-go/pkg/adapter"
)

// Write an event to the server, and return an error if it fails
func (i *Client) Write(ev *adapter.Event) error {
	var payload [][]byte
	log.Printf("Writing Event: %#v", ev)

	payload = append(payload, []byte(ev.Command))
	for i, p := range ev.Parameters {
		if i == len(ev.Parameters)-1 && len(ev.Parameters) > 1 {
			//What was the point of this? it causes errors...
			//p = fmt.Sprintf(":%s\r\n", p)
			p = fmt.Sprintf("%s\r\n", p)
		}
		payload = append(payload, []byte(p))
	}

	payload = append(payload, []byte("\r\n"))
	full := bytes.Join(payload, []byte(" "))
	_, err := i.conn.Write(full)
	return err
}
