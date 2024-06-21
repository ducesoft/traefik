package tcp

import (
	"context"
	"github.com/traefik/traefik/v3/pkg/middlewares/accesslog"
	"net"
)

// Handler is the TCP Handlers interface.
type Handler interface {
	ServeTCP(conn WriteCloser)
}

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as handlers.
type HandlerFunc func(conn WriteCloser)

// ServeTCP serves tcp.
func (f HandlerFunc) ServeTCP(conn WriteCloser) {
	f(conn)
}

// WriteCloser describes a net.Conn with a CloseWrite method.
type WriteCloser interface {
	Conn

	// Context is the tcp context.
	Context() context.Context
}

// Conn describes a net.Conn with a CloseWrite method.
type Conn interface {
	net.Conn
	// CloseWrite on a network connection, indicates that the issuer of the call
	// has terminated sending on that connection.
	// It corresponds to sending a FIN packet.
	CloseWrite() error
}

func StatefulConn(conn Conn) WriteCloser {
	if x, ok := conn.(WriteCloser); ok {
		return x
	}
	return &StatefulWriteCloser{
		Conn: conn,
		ctx:  contextProvider.New(),
	}
}

type StatefulWriteCloser struct {
	Conn
	ctx context.Context
}

func (that *StatefulWriteCloser) Context() context.Context {
	return that.ctx
}

func NewFieldHandler(h Handler, kvs map[string]any) Handler {
	return &FieldHandler{h: h, kvs: kvs}
}

type FieldHandler struct {
	h   Handler
	kvs map[string]any
}

func (that *FieldHandler) ServeTCP(conn WriteCloser) {
	for k, v := range that.kvs {
		contextProvider.Set(conn.Context(), k, v)
	}
	that.h.ServeTCP(conn)
}

func AsContextProvider(cp ContextProvider) {
	contextProvider = cp
}

func SetContextState(ctx context.Context, key string, value any) {
	contextProvider.Set(ctx, key, value)
}

func GetContextState(ctx context.Context, key string) any {
	return contextProvider.Get(ctx, key)
}

var contextProvider ContextProvider = new(dftContextProvider)

type ContextProvider interface {
	New() context.Context
	Set(ctx context.Context, key string, value any)
	Get(ctx context.Context, key string) any
}

const statefulState = "traefik.tcp"

type dftContextProvider struct {
}

func (that *dftContextProvider) New() context.Context {
	return context.WithValue(context.Background(), statefulState, map[string]any{
		accesslog.RequestProtocol: "TCP",
	})
}

func (that *dftContextProvider) Set(ctx context.Context, key string, value any) {
	if x, ok := ctx.Value(statefulState).(map[string]any); ok {
		x[key] = value
	}
}

func (that *dftContextProvider) Get(ctx context.Context, key string) any {
	if x, ok := ctx.Value(statefulState).(map[string]any); ok {
		return x[key]
	}
	return nil
}
