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
	"sort"

	"github.com/rs/zerolog/log"
)

var (
	filters   = map[string]Filter{}
	tlsFilter TLSFilter
)

type Filter interface {

	// Name is middleware name
	Name() string

	// Priority more than has more priority
	Priority() int

	// Scope is middleware effect scope, 0 is global, 1 is customized, 2 is alp tcp.
	Scope() int

	// New middleware instance
	New(ctx context.Context, next Handler, name string) (Handler, error)
}

type NextFilter interface {
	Filter
	// Next middleware instance
	Next(ctx context.Context, next Handler, name string, option any) (Handler, error)
}

type TLSFilter interface {
	// ServeTCP tls
	ServeTCP(conn WriteCloser, next Handler, config *tls.Config, plugin map[string]any, forwarder Handler) (WriteCloser, bool)
}

// Provide the middleware
func Provide(filter Filter) {
	filters[filter.Name()] = filter
}

// ProvideTLS the middleware
func ProvideTLS(filter TLSFilter) {
	tlsFilter = filter
}

func WithFilter(name string, fn func(filter Filter)) {
	if x := filters[name]; nil != x {
		fn(x)
	}
}

func WithNextFilter(name string, fn func(m NextFilter)) {
	WithFilter(name, func(f Filter) {
		if next, ok := f.(NextFilter); ok {
			fn(next)
		}
	})
}

func GlobalFilters(ctx context.Context) Constructor {
	return scopedFilters(ctx, 0)
}

func scopedFilters(ctx context.Context, scope int) Constructor {
	var fs []Filter
	for _, f := range filters {
		if f.Scope() == scope {
			fs = append(fs, f)
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Priority() < fs[j].Priority() })
	constructor := func(next Handler) (Handler, error) {
		var err error
		for _, filter := range fs {
			if n, ok := filter.(NextFilter); ok {
				if next, err = n.Next(ctx, next, filter.Name(), nil); nil != err {
					return nil, err
				}
				continue
			}
			if next, err = filter.New(ctx, next, filter.Name()); nil != err {
				return nil, err
			}
		}
		return next, nil
	}
	return constructor
}

func IsTimeout(err error) bool {
	if nil == err {
		return false
	}
	switch x := err.(type) {
	case nil:
		return false
	case *net.OpError:
		return x.Timeout()
	default:
		return false
	}
}

type Hello interface {
	// ServerName is SNI server name
	ServerName() string

	// Protos is ALPN protocols list
	Protos() []string

	// IsTLS is whether we are a TLS handshake
	IsTLS() bool

	// Peeked the bytes peeked from the hello while getting the info
	Peeked() string
}

func NewALPTCPChain(h Handler) Handler {
	hh, err := NewChain(scopedFilters(contextProvider.New(), 2)).Then(h)
	if nil != err {
		log.Error().Err(err)
		return h
	}
	return hh
}
