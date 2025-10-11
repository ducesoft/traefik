/*
 * Copyright (c) 2019, 2025, firmer.tech and/or its affiliates. All rights reserved.
 * Firmer Corporation PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package tcp

import (
	"context"
	"net"
)

const (
	RequestClientAddr = "RequestClientAddr"
	RequestServerAddr = "RequestServerAddr"
	ProxyClientAddr   = "ProxyClientAddr"
	ProxyServerAddr   = "ProxyServerAddr"
)

func NewNextConn(conn WriteCloser) NextConn {
	return NewNextConnWithContext(contextProvider.New(), conn)
}

func NewNextConnWithContext(ctx context.Context, conn WriteCloser) NextConn {
	return &nextConn{WriteCloser: conn, ctx: ctx}
}

type NextConn interface {
	WriteCloser
	Context() context.Context
}

type nextConn struct {
	WriteCloser
	ctx context.Context
}

func (that *nextConn) Context() context.Context {
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
	if next, ok := conn.(NextConn); ok {
		contextProvider.Set(next.Context(), that.kvs)
	}
	that.h.ServeTCP(conn)
}

func ProvideContext(cp ContextProvider) {
	contextProvider = cp
}

func ContextVars(conn WriteCloser, kvs map[string]any) {
	if next, ok := conn.(NextConn); ok {
		contextProvider.Set(next.Context(), kvs)
	}
}

func ContextVar(ctx net.Conn, key string) any {
	if next, ok := ctx.(NextConn); ok {
		return contextProvider.Get(next.Context(), key)
	}
	return nil
}

var contextProvider ContextProvider = new(dftContextProvider)

type ContextProvider interface {
	New() context.Context
	Set(ctx context.Context, kvs map[string]any)
	Get(ctx context.Context, key string) any
}

const statefulState = "traefik.tcp"

type dftContextProvider struct {
}

func (that *dftContextProvider) New() context.Context {
	return context.WithValue(context.Background(), statefulState, map[string]any{
		// accesslog.RequestProtocol
		"RequestProtocol": "TCP",
	})
}

func (that *dftContextProvider) Set(ctx context.Context, kvs map[string]any) {
	if x, ok := ctx.Value(statefulState).(map[string]any); ok {
		for k, v := range kvs {
			x[k] = v
		}
	}
}

func (that *dftContextProvider) Get(ctx context.Context, key string) any {
	if x, ok := ctx.Value(statefulState).(map[string]any); ok {
		return x[key]
	}
	return nil
}
