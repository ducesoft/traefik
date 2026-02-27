package udp

import (
	"io"
	"net"
)

// Connection allows middleware to wrap a session while preserving datagram boundaries.
type Connection interface {
	io.ReadWriteCloser
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// Handler is the UDP counterpart of the usual HTTP handler.
type Handler interface {
	ServeUDP(conn Connection)
}

// The HandlerFunc type is an adapter to allow the use of ordinary functions as handlers.
type HandlerFunc func(conn Connection)

// ServeUDP implements the Handler interface for UDP.
func (f HandlerFunc) ServeUDP(conn Connection) {
	f(conn)
}
