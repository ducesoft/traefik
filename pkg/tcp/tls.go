package tcp

import (
	"context"
	"crypto/tls"
	"net"

	traefiktls "github.com/traefik/traefik/v3/pkg/tls"
)

// TLSConn is a TLS connection that also carries the name of the TLS config used.
type TLSConn struct {
	WriteCloser

	TLSOptionsName string
}

func (t *TLSConn) Raw() net.Conn {
	return t.WriteCloser
}

func (t *TLSConn) Unwrap() net.Conn {
	return t.WriteCloser
}

// TLSHandler handles TLS connections.
type TLSHandler struct {
	Next           Handler
	Config         *tls.Config
	TLSOptionsName string
	//
	Plugin    map[string]any
	Forwarder Handler
}

// ServeTCP terminates the TLS connection.
func (t *TLSHandler) ServeTCP(conn WriteCloser) {
	cc := TLSConn{WriteCloser: conn, TLSOptionsName: t.TLSOptionsName}
	if nil == tlsFilter {
		t.Next.ServeTCP(tls.Server(cc, t.Config))
		return
	}
	rc, ok := tlsFilter.ServeTCP(cc, t.Next, t.Config, t.Plugin, t.Forwarder)
	if !ok {
		t.Next.ServeTCP(tls.Server(rc, t.Config))
	}
}

type tlsOptionsNameKey struct{}

func AddTLSOptionsNameInContext(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, tlsOptionsNameKey{}, name)
}

func GetTLSOptionsName(ctx context.Context) string {
	if name, ok := ctx.Value(tlsOptionsNameKey{}).(string); ok {
		return name
	}

	return ""
}

func TLSServer(next Handler, config *tls.Config, optionsName string, plugin map[string]any, forwarder Handler) Handler {
	return NewFieldFnHandler(&TLSHandler{
		Next:           next,
		Config:         config,
		TLSOptionsName: optionsName,
		Plugin:         plugin,
		Forwarder:      forwarder,
	}, func(ctx context.Context, conn WriteCloser) map[string]string {
		if tc, ok := conn.(*tls.Conn); ok {
			state := tc.ConnectionState()
			return map[string]string{
				RequestTLSVersion: traefiktls.GetVersion(&state),
				RequestTLSCipher:  traefiktls.GetCipherName(&state),
				RequestTLSSNI:     state.ServerName,
				RequestProtocol:   "TLS",
				RequestTLSOption:  optionsName,
			}
		}
		return map[string]string{}
	})
}
