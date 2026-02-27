/*
 * Copyright (c) 2019, 2025, firmer.tech and/or its affiliates. All rights reserved.
 * Firmer Corporation PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package tcp

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	RequestClientAddr = "rc_client_addr"
	RequestServerAddr = "rc_server_addr"
	RequestTLSVersion = "rc_tls_version"
	RequestTLSCipher  = "rc_tls_cipher"
	RequestTLSSNI     = "rc_tls_sni"
	RequestProtocol   = "rc_protocol"
	ProxyClientAddr   = "pc_client_addr"
	ProxyServerAddr   = "pc_server_addr"
	ProxyTLSVersion   = "rc_tls_version"
	ProxyTLSCipher    = "rc_tls_cipher"
	ProxyTLSSNI       = "rc_tls_sni"
	ProxyProtocol     = "pc_protocol"
	ServiceURL        = "service_url"
	ServiceName       = "service_name"
	ServiceAddr       = "service_addr"
	RouterName        = "router_name"
	Timestamp         = "timestamp"
	Status            = "status"
	TraceID           = "trace_id"
)

func NewNextConn(conn WriteCloser) NextConn {
	return NewNextConnWithContext(contextProvider.New(), conn)
}

func NewNextConnWithContext(ctx context.Context, conn WriteCloser) NextConn {
	if nc, ok := conn.(NextConn); ok {
		return nc
	}
	contextProvider.Set(ctx, map[string]string{
		RequestClientAddr: conn.RemoteAddr().String(),
		RequestServerAddr: conn.LocalAddr().String(),
		Timestamp:         strconv.FormatInt(time.Now().UnixMilli(), 10),
	})
	if tc, ok := conn.(tlsConn); ok {
		return &nextTLSConn{
			tlsConn: tc,
			ctx:     ctx,
			closed:  atomic.Bool{},
		}
	}
	return &nextConn{
		WriteCloser: conn,
		ctx:         ctx,
		closed:      atomic.Bool{},
	}
}

type tlsConn interface {
	WriteCloser
	NetConn() net.Conn
	Handshake() error
	HandshakeContext(ctx context.Context) error
	ConnectionState() tls.ConnectionState // must implement
	OCSPResponse() []byte
	VerifyHostname(host string) error
}

type NextConn interface {
	Raw() net.Conn
	Context() context.Context
	WriteCloser
}

type nextConn struct {
	WriteCloser
	ctx    context.Context
	closed atomic.Bool
}

func (that *nextConn) Raw() net.Conn {
	return that.WriteCloser
}

func (that *nextConn) Context() context.Context {
	return that.ctx
}

func (that *nextConn) Close() error {
	if that.closed.Load() {
		return nil
	}
	that.closed.Store(true)
	return that.WriteCloser.Close()
}

type nextTLSConn struct {
	tlsConn
	ctx    context.Context
	closed atomic.Bool
}

func (that *nextTLSConn) Raw() net.Conn {
	return that.tlsConn
}

func (that *nextTLSConn) Context() context.Context {
	return that.ctx
}

func (that *nextTLSConn) Close() error {
	if that.closed.Load() {
		return nil
	}
	that.closed.Store(true)
	return that.tlsConn.Close()
}

func NewFieldHandler(h Handler, kvs map[string]string) Handler {
	return NewFieldFnHandler(h, func(ctx context.Context, conn WriteCloser) map[string]string {
		return kvs
	})
}

func NewFieldFnHandler(h Handler, kvs func(ctx context.Context, conn WriteCloser) map[string]string) Handler {
	return &FieldHandler{h: h, kvs: kvs}
}

type FieldHandler struct {
	h   Handler
	kvs func(ctx context.Context, conn WriteCloser) map[string]string
}

func (that *FieldHandler) ServeTCP(conn WriteCloser) {
	that.h.ServeTCP(conn)
	if next, ok := conn.(NextConn); ok {
		contextProvider.Set(next.Context(), that.kvs(next.Context(), conn))
	}
}

func ProvideContext(cp ContextProvider) {
	contextProvider = cp
}

func ContextVars(conn WriteCloser, kvs ...map[string]string) map[string]string {
	next, ok := conn.(NextConn)
	if !ok {
		return map[string]string{}
	}
	for _, kv := range kvs {
		contextProvider.Set(next.Context(), kv)
	}
	if v := contextProvider.Get(next.Context()); nil != v {
		return v
	}
	return map[string]string{}
}

var contextProvider ContextProvider = new(dftContextProvider)

type ContextProvider interface {
	New() context.Context
	Set(ctx context.Context, kvs map[string]string)
	Get(ctx context.Context) map[string]string
}

const statefulState = "traefik.tcp"

type dftContextProvider struct {
}

func (that *dftContextProvider) New() context.Context {
	return context.WithValue(context.Background(), statefulState, map[string]string{
		// accesslog.RequestProtocol
		RequestProtocol: "TCP",
	})
}

func (that *dftContextProvider) Set(ctx context.Context, kvs map[string]string) {
	if x, ok := ctx.Value(statefulState).(map[string]string); ok {
		for k, v := range kvs {
			x[k] = v
		}
	}
}

func (that *dftContextProvider) Get(ctx context.Context) map[string]string {
	if x, ok := ctx.Value(statefulState).(map[string]string); ok {
		return x
	}
	return nil
}
