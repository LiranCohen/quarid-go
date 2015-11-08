// Package irc provides IRC client services in Golang
//
// About
//
// This package implements an simple IRC service, that can be used in Golang to
// build IRC clients, bots, or other tools.
//
// See also: https://tools.ietf.org/html/rfc2812
package irc

import (
	"net"
	"time"

	"github.com/enmand/quarid-go/pkg/adapter"
)

// TIMEOUT is the connection timeout to the IRC server
const TIMEOUT = 1 * time.Minute

// IRC is the IRC client interface
type IRC interface {
	// Connect to an IRC server. Use the form address:port
	Connect(server string) error

	// Disconnect from an IRC server
	Disconnect() error

	// Loop blocks while reading from the server
	Loop()

	adapter.EventsHandler
	adapter.Responder
}

// Client is the implementation of the IRC interface
type Client struct {
	// The client's nickname on the server
	Nick string

	Password string
	// The client's password on the server

	// The client's Ident on the server
	Ident string

	// The client's hostname
	Host string

	// The client's masked hostname on the server (if masked)
	MaskedHost string

	// If this connection is a TLS connection
	TLS bool

	// Should this client verify the server's SSL certs
	TLSVerify bool

	// handlers for filtered events
	handlers []*adapter.Handler

	// Dead is blocks until the conn
	dead chan bool

	// Events broadcasted from the server
	events chan *adapter.Event

	// The network connection this client has to the server
	conn net.Conn
}

// NewClient returns a new IRC client
func NewClient(nick, ident string, tlsverify, tls bool) *Client {
	return &Client{
		Nick:      nick,
		Ident:     ident,
		TLSVerify: tlsverify,
		TLS:       tls,
	}
}
