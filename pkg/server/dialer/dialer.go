/*
 * Copyright (c) 2019, 2025, firmer.tech and/or its affiliates. All rights reserved.
 * Firmer Corporation PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package dialer

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"golang.org/x/net/proxy"
)

func init() {
	Provide(new(proxyNextDialer))
	Provide(new(proxyTLSDialer))
}

const (
	ALP                  = "ALP"
	TCP                  = "TCP"
	proxyProtoClientConn = "proxyProtoClientConn"
)

func NewProxyProtoContext(conn ClientConn) context.Context {
	return context.WithValue(context.Background(), proxyProtoClientConn, conn)
}

func NewTCPDialer(tcp *dynamic.TCPServersTransport, d proxy.Dialer) Dialer {
	return NewDialer(context.Background(), WithDialer(d), WithTCP(tcp))
}

func NewTLSDialer(tcp *dynamic.TCPServersTransport, config *tls.Config, d proxy.Dialer) Dialer {
	return NewDialer(context.Background(), WithDialer(d), WithTCP(tcp), WithTLS(config))
}

func NewTLSProxyProtoDialer(tcp *dynamic.TCPServersTransport, config *tls.Config, d ProxyProtoDialer) Dialer {
	return NewDialer(context.Background(), WithProxyProtoDialer(d), WithTCP(tcp), WithTLS(config))
}

func NewHTTPDialer(tcp *dynamic.ServersTransport, d proxy.Dialer) Dialer {
	return NewDialer(context.Background(), WithDialer(d), WithALP(tcp))
}

func NewHTTPSDialer(tcp *dynamic.ServersTransport, config *tls.Config, d proxy.Dialer) Dialer {
	return NewDialer(context.Background(), WithDialer(d), WithALP(tcp), WithTLS(config))
}

func NewHTTPProxy(tcp *dynamic.ServersTransport, d proxy.Dialer) func(req *http.Request) (*url.URL, error) {
	return NewProxy(context.Background(), WithDialer(d), WithALP(tcp))
}

func NewDialer(ctx context.Context, options ...Fn) Dialer {
	d := &dialer{underlay: &net.Dialer{}}
	for _, o := range options {
		o(d)
	}
	for _, connector := range connectors {
		if connector.Match(ctx, d) {
			d.overlays = append(d.overlays, connector)
		}
	}
	if nil != d.proxyProto {
		underlay := d.underlay
		d.underlay = FnContextDialer(func(ctx context.Context, network, addr string) (net.Conn, error) {
			if c, ok := ctx.Value(proxyProtoClientConn).(ClientConn); ok {
				return d.proxyProto.Dial(network, addr, c)
			}
			return underlay.DialContext(ctx, network, addr)
		})
	}
	return d
}

func NewProxy(ctx context.Context, options ...Fn) Proxy {
	d := &dialer{skips: map[string]bool{}}
	for _, o := range options {
		o(d)
	}
	p := http.ProxyFromEnvironment
	for _, connector := range connectors {
		if connector.Match(ctx, d) {
			p = connector.Proxy(ctx, d, p)
		}
	}
	return p
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
		option.proto = TCP
		option.plugin = tcp.Plugin
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
		option.proto = ALP
		option.plugin = tcp.Plugin
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

func WithProxyProtoDialer(d ProxyProtoDialer) Fn {
	return func(option *dialer) {
		option.proxyProto = d
	}
}

func WithTLS(c *tls.Config) Fn {
	return func(option *dialer) {
		option.tls = c
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
	Proto() string
	Proxy() *url.URL
	ServerName() string
	TLS() *tls.Config
	Skip(name string, override bool) bool
	Plugin(name string) any
	Clone() Option
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
	if nil == option.Proxy() || strings.HasPrefix(option.Proxy().Scheme, "http") || (option.Proto() == ALP && option.Proxy().Query().Get("v") != "4") {
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
	if nil == option.Proxy() || option.Proxy().Query().Get("v") == "4" {
		return proxy
	}
	return http.ProxyURL(option.Proxy())
}

type Fn func(d *dialer)

type Dialer interface {
	proxy.Dialer
	proxy.ContextDialer
}

// ClientConn is the interface that provides information about the client connection.
type ClientConn interface {
	// LocalAddr returns the local network address, if known.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address, if known.
	RemoteAddr() net.Addr
}

// ProxyProtoDialer is an interface to dial a network connection, with support for PROXY protocol and termination delay.
type ProxyProtoDialer interface {
	Dial(network, addr string, clientConn ClientConn) (c net.Conn, err error)
	TerminationDelay() time.Duration
}

type FnContextDialer func(ctx context.Context, network, addr string) (net.Conn, error)

func (that FnContextDialer) Dial(network, addr string) (net.Conn, error) {
	return that.DialContext(context.Background(), network, addr)
}

func (that FnContextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return that(ctx, network, address)
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
	ref        Dialer // cache the ref dialer
	proto      string
	proxy      *url.URL
	serverName string
	underlay   Dialer
	proxyProto ProxyProtoDialer
	overlays   []NextDialer
	tls        *tls.Config
	skips      map[string]bool
	plugin     map[string]any
}

func (that *dialer) Proto() string {
	return that.proto
}

func (that *dialer) Proxy() *url.URL {
	return that.proxy
}

func (that *dialer) ServerName() string {
	return that.serverName
}

func (that *dialer) TLS() *tls.Config {
	return that.tls
}

func (that *dialer) Skip(name string, override bool) bool {
	if override {
		that.skips[name] = true
		return true
	}
	return that.skips[name]
}

func (that *dialer) Plugin(name string) any {
	if nil == that.plugin {
		return nil
	}
	return that.plugin[name]
}

func (that *dialer) Clone() Option {
	return &dialer{
		ref:        that.ref,
		proto:      that.proto,
		proxy:      that.proxy,
		serverName: that.serverName,
		underlay:   that.underlay,
		proxyProto: that.proxyProto,
		overlays:   that.overlays,
		tls:        that.tls,
		skips:      map[string]bool{},
		plugin:     that.plugin,
	}
}

func (that *dialer) Dial(network, addr string) (net.Conn, error) {
	return that.DialContext(context.Background(), network, addr)
}

func (that *dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if nil != that.ref {
		return that.ref.DialContext(ctx, network, address)
	}
	if len(that.overlays) < 1 {
		return that.underlay.DialContext(ctx, network, address)
	}
	option := that.Clone()
	d := that.underlay
	for _, overlay := range that.overlays {
		d = overlay.Next(ctx, option, d)
	}
	that.ref = d
	return d.DialContext(ctx, network, address)
}

type proxyTLSDialer struct {
	d Dialer
}

func (that *proxyTLSDialer) Priority() int {
	return math.MaxInt - 500
}

func (that *proxyTLSDialer) Match(ctx context.Context, option Option) bool {
	return nil != option && nil != option.TLS()
}

func (that *proxyTLSDialer) Next(ctx context.Context, option Option, dialer Dialer) Dialer {
	return &tlsDialer{d: dialer, o: option}
}

func (that *proxyTLSDialer) Proxy(ctx context.Context, option Option, proxy Proxy) Proxy {
	return proxy
}

type tlsDialer struct {
	d Dialer
	o Option
}

func (that *tlsDialer) Dial(network, addr string) (c net.Conn, err error) {
	return that.DialContext(context.Background(), network, addr)
}

func (that *tlsDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if that.o.Skip("tls", false) {
		return that.d.DialContext(ctx, network, address)
	}
	host, _, err := net.SplitHostPort(address)
	if nil != err {
		return nil, err
	}
	c, err := that.d.DialContext(ctx, network, address)
	if nil != err {
		return nil, err
	}
	config := that.o.TLS().Clone()
	if config.ServerName == "" {
		config.ServerName = host
	}
	log.Debug().Msgf("tls handshake to %s with server name %s", address, config.ServerName)

	tlsConn := tls.Client(c, config)
	if err = tlsConn.HandshakeContext(ctx); nil != err {
		c.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}
	return tlsConn, nil
}
