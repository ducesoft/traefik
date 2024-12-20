/*
 * Copyright (c) 2019, 2023, ducesoft and/or its affiliates. All rights reserved.
 * DUCESOFT PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package tcp

import (
	"context"
	"net"
	"sort"
)

var filters = map[string]Filter{}

type Filter interface {

	// Name is middleware name
	Name() string

	// Priority more than has more priority
	Priority() int

	// Scope is middleware effect scope, 0 is global, others is customized.
	Scope() int

	// New middleware instance
	New(ctx context.Context, next Handler, name string) (Handler, error)
}

// Provide the middleware
func Provide(filter Filter) {
	filters[filter.Name()] = filter
}

func WithFilter(name string, fn func(filter Filter)) {
	if x := filters[name]; nil != x {
		fn(x)
	}
}

func GlobalFilters(ctx context.Context) Constructor {
	var fs []Filter
	for _, f := range filters {
		if f.Scope() == 0 {
			fs = append(fs, f)
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Priority() < fs[j].Priority() })
	constructor := func(next Handler) (Handler, error) {
		var err error
		for _, filter := range fs {
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
