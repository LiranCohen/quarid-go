package irc

// Responder
//
// Responder responds to the IRC server, by writing an IRC Event to the CLient's
// connection

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/enmand/quarid-go/pkg/adapter"
	"github.com/enmand/quarid-go/pkg/logger"
)

// Write an event to the server, and return an error if it fails
func (i *Client) Write(ev *adapter.Event) error {
	var payload [][]byte
	logger.Log.Info("Writing event: ", ev)

	payload = append(payload, []byte(ev.Command))
	for i, p := range ev.Parameters {
		if i == len(ev.Parameters)-1 {
			multi := strings.Split(p, " ")
			if len(multi) > 1 {
				p = fmt.Sprintf(":%s", p)
			}
		}
		payload = append(payload, []byte(p))
	}

	payload = append(payload, []byte("\r\n"))
	full := bytes.Join(payload, []byte(" "))

	_, err := i.conn.Write(full)
	return err
}
