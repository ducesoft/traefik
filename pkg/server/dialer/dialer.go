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
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
)

func init() {
	Provide(new(proxyNextDialer))
}

func NewDialer(ctx context.Context, options ...Fn) Dialer {
	d := &dialer{}
	for _, o := range options {
		o(d)
	}
	for _, connector := range connectors {
		if connector.Match(ctx, d) {
			d.overlays = append(d.overlays, connector)
		}
	}
	return d
}

func NewProxy(ctx context.Context, options ...Fn) Proxy {
	//d := &dialer{}
	//for _, o := range options {
	//	o(d)
	//}
	//for _, connector := range connectors {
	//	if connector.Match(ctx, d) {
	//		d.overlays = append(d.overlays, connector)
	//	}
	//}
	//return d
	return http.ProxyFromEnvironment
}

func Provide(dialer NextDialer) {
	connectors = append(connectors, dialer)
	sort.SliceStable(connectors, func(i, j int) bool {
		return connectors[i].Priority() > connectors[j].Priority()
	})
}

func WithProxy(proxy string) Fn {
	return func(option *dialer) {
		if "" == proxy {
			return
		}
		uri, err := url.Parse(proxy)
		if nil != err {
			log.Error().Msgf("Error while create transport proxy, %v", err)
			return
		}
		option.proxy = uri
	}
}

func WithTCP(tcp *dynamic.TCPServersTransport) Fn {
	return func(option *dialer) {
		if nil != tcp {
			WithProxy(tcp.Proxy)(option)
		}
		if nil != tcp && nil != tcp.TLS {
			option.serverName = tcp.TLS.ServerName
		}
	}
}

func WithALP(tcp *dynamic.ServersTransport) Fn {
	return func(option *dialer) {
		if nil != tcp {
			WithProxy(tcp.Proxy)(option)
			option.serverName = tcp.ServerName
		}
	}
}

func WithDialer(d proxy.Dialer) Fn {
	return func(option *dialer) {
		option.underlay = &proxyDialer{d: d}
	}
}

func WithContextDialer(d proxy.ContextDialer) Fn {
	return func(option *dialer) {
		option.underlay = &contextDialer{d: d}
	}
}

var (
	_          Dialer = new(dialer)
	connectors []NextDialer
)

type Option interface {
	Proxy() *url.URL
	ServerName() string
}

type Proxy func(req *http.Request) (*url.URL, error)

type NextDialer interface {

	// Priority max will be Dial first
	Priority() int

	// Match the provider
	Match(ctx context.Context, option Option) bool

	// Next a dialer
	Next(ctx context.Context, option Option, dialer Dialer) Dialer

	// Proxy a dialer
	Proxy(ctx context.Context, option Option, proxy Proxy) Proxy
}

var _ NextDialer = new(proxyNextDialer)

type proxyNextDialer struct {
}

func (that *proxyNextDialer) Priority() int {
	return math.MaxInt
}

func (that *proxyNextDialer) Match(ctx context.Context, option Option) bool {
	return true
}

func (that *proxyNextDialer) Next(ctx context.Context, option Option, dialer Dialer) Dialer {
	if nil == option.Proxy() {
		return &proxyDialer{d: proxy.FromEnvironmentUsing(dialer)}
	}
	socks5, err := proxy.FromURL(option.Proxy(), dialer)
	if nil != err {
		log.Error().Msgf("Error while create transport proxy, %v", err)
		return &proxyDialer{d: proxy.FromEnvironmentUsing(dialer)}
	}
	return &proxyDialer{d: socks5}
}

func (that *proxyNextDialer) Proxy(ctx context.Context, option Option, proxy Proxy) Proxy {
	if nil == option.Proxy() {
		return proxy
	}
	return proxy
}

type Fn func(d *dialer)

type Dialer interface {
	proxy.Dialer
	proxy.ContextDialer
}

type proxyDialer struct {
	d proxy.Dialer
}

func (that *proxyDialer) Dial(network, addr string) (net.Conn, error) {
	return that.d.Dial(network, addr)
}

func (that *proxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return that.Dial(network, address)
}

type contextDialer struct {
	d proxy.ContextDialer
}

func (that *contextDialer) Dial(network, addr string) (net.Conn, error) {
	return that.DialContext(context.Background(), network, addr)
}

func (that *contextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return that.d.DialContext(ctx, network, address)
}

type dialer struct {
	proxy      *url.URL
	serverName string
	underlay   Dialer
	overlays   []NextDialer
}

func (that *dialer) Proxy() *url.URL {
	return that.proxy
}

func (that *dialer) ServerName() string {
	return that.serverName
}

func (that *dialer) Dial(network, addr string) (net.Conn, error) {
	return that.DialContext(context.Background(), network, addr)
}

func (that *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if len(that.overlays) < 1 {
		return that.underlay.DialContext(ctx, network, address)
	}
	d := that.underlay
	for _, overlay := range that.overlays {
		d = overlay.Next(ctx, that, d)
	}
	return d.DialContext(ctx, network, address)
}

//func x() {
//	uri, err := url.Parse(tcp.Proxy)
//	if nil != err {
//		log.Error().Msgf("Error while create transport proxy, %v", err)
//		return http.ProxyFromEnvironment
//	}
//	if netDialer, ok := netsDialer[uri.Scheme]; ok && nil != netDialer {
//		return http.ProxyFromEnvironment
//	}
//	name := uri.Query().Get("n")
//	if "" == name {
//		return http.ProxyURL(uri)
//	}
//	if proxies, ok := httpProxies[name]; ok && nil != proxies {
//		query := uri.Query()
//		query.Del("n")
//		uri.RawQuery = query.Encode()
//		return proxies.New(uri.String()).Proxy
//	}
//	return http.ProxyFromEnvironment
//}
