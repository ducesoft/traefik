package tcp

import (
	"context"
	"crypto/tls"

	traefiktls "github.com/traefik/traefik/v3/pkg/tls"
)

// TLSHandler handles TLS connections.
type TLSHandler struct {
	Next      Handler
	Config    *tls.Config
	Plugin    map[string]any
	Forwarder Handler
}

// ServeTCP terminates the TLS connection.
func (t *TLSHandler) ServeTCP(conn WriteCloser) {
	if nil == tlsFilter {
		t.Next.ServeTCP(tls.Server(conn, t.Config))
		return
	}
	rc, ok := tlsFilter.ServeTCP(conn, t.Next, t.Config, t.Plugin, t.Forwarder)
	if !ok {
		t.Next.ServeTCP(tls.Server(rc, t.Config))
	}
}

func TLSServer(next Handler, config *tls.Config, plugin map[string]any, forwarder Handler) Handler {
	return NewFieldFnHandler(&TLSHandler{Next: next, Config: config, Plugin: plugin, Forwarder: forwarder}, func(ctx context.Context, conn WriteCloser) map[string]string {
		if tc, ok := conn.(*tls.Conn); ok {
			state := tc.ConnectionState()
			return map[string]string{
				RequestTLSVersion: traefiktls.GetVersion(&state),
				RequestTLSCipher:  traefiktls.GetCipherName(&state),
				RequestTLSSNI:     state.ServerName,
				RequestProtocol:   "TLS",
			}
		}
		return map[string]string{}
	})
}
