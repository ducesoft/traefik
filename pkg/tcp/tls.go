package tcp

import (
	"crypto/tls"
)

// TLSHandler handles TLS connections.
type TLSHandler struct {
	Next   Handler
	Config *tls.Config
	Plugin map[string]any
}

// ServeTCP terminates the TLS connection.
func (t *TLSHandler) ServeTCP(conn WriteCloser) {
	if nil == tlsFilter {
		t.Next.ServeTCP(tls.Server(conn, t.Config))
		return
	}
	rc, ok := tlsFilter.ServeTCP(conn, t.Next, t.Config, t.Plugin)
	if !ok {
		t.Next.ServeTCP(tls.Server(rc, t.Config))
	}
}

func TLSServer(next Handler, config *tls.Config, plugin map[string]any) Handler {
	return &TLSHandler{Next: next, Config: config, Plugin: plugin}
}
