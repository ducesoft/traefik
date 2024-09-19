/*
 * Copyright (c) 2019, 2023, ducesoft and/or its affiliates. All rights reserved.
 * DUCESOFT PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package dialer

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"golang.org/x/net/proxy"
	"net"
	"net/url"
)

func NewDialer(ctx context.Context, options ...Option) Dialer {
	v := d
	for _, o := range options {
		if n, err := o(ctx, d); nil != err {
			log.Error().Msgf("Error while create transport proxy, %v", err)
		} else {
			v = n.Next(ctx, v)
		}
	}
	return v
}

func Provide(name string, dialer NextDialer) {
	dials[name] = dialer
}

func WithDialer(d proxy.Dialer)Option {

}

func WithString(name string) Option {
	return func(ctx context.Context) (NextDialer, error) {
		if d, ok := dials[name]; ok && nil != d {
			return d.Next(ctx, d), nil
		}
	}
}

func WithURL(uri string) Option {
	return func(d *dialer) error {
		u, err := url.Parse(uri)
		if nil != err {
			return err
		}
		d.names = append(d.names, u.Scheme)
		return nil
	}
}

func WithTCP(tcp *dynamic.TCPServersTransport) Option {
	return func(d *dialer) error {
		if nil != tcp && nil != tcp.TLS {
			d.names = append(d.names, tcp.TLS.ServerName)
		}
		return nil
	}
}

func WithProxy(proxy string) Option {
	return func(d *dialer) error {
		d.names = append(d.names, proxy)
		return nil
	}

}

var (
	_     Dialer = new(dialer)
	dials        = map[string]NextDialer{}
)

type Option func(ctx context.Context, d Dialer) (NextDialer, error)

type Dialer interface {
	proxy.Dialer
	proxy.ContextDialer
}

type dialer struct {
	names []string
	d     Dialer
}

func (that *dialer) Dial(network, addr string) (net.Conn, error) {
	return that.d.Dial(network, addr)
}

func (that *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return that.d.DialContext(ctx, network, address)
}

type NextDialer interface {
	Next(ctx context.Context, dialer Dialer) Dialer
}
