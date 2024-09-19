package service

import (
	"context"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
)

const DefaultName = "default"

var (
	httpProxies = map[string]Proxies{}
	netsDialer  = map[string]ContextDialer{}
)

func ProvideDialer(dialer ContextDialer) {
	netsDialer[dialer.Name()] = dialer
}

func ProvideProxy(proxies Proxies) {
	httpProxies[proxies.Name()] = proxies
}

func CreateDialer(proxy string, serverName string, dialer *net.Dialer) func(ctx context.Context, network string, address string) (net.Conn, error) {
	if "" != proxy {
		uri, err := url.Parse(proxy)
		if nil == err {
			if netDialer, ok := netsDialer[uri.Scheme]; ok && nil != netDialer {
				return netDialer.New(proxy, serverName, dialer).DialContext
			}
		} else {
			log.Error().Msgf("Error while create transport proxy, %v", err)
		}
	}
	if "" == serverName {
		return dialer.DialContext
	}
	if netDialer, ok := netsDialer[serverName]; ok && nil != netDialer {
		return netDialer.New(proxy, serverName, dialer).DialContext
	}
	if netDialer, ok := netsDialer[DefaultName]; ok && nil != netDialer {
		return netDialer.New(proxy, serverName, dialer).DialContext
	}
	return dialer.DialContext
}

func CreateProxy(proxy string) func(req *http.Request) (*url.URL, error) {
	if "" == proxy {
		return http.ProxyFromEnvironment
	}
	uri, err := url.Parse(proxy)
	if nil != err {
		log.Error().Msgf("Error while create transport proxy, %v", err)
		return http.ProxyFromEnvironment
	}
	if netDialer, ok := netsDialer[uri.Scheme]; ok && nil != netDialer {
		return http.ProxyFromEnvironment
	}
	name := uri.Query().Get("n")
	if "" == name {
		return http.ProxyURL(uri)
	}
	if proxies, ok := httpProxies[name]; ok && nil != proxies {
		query := uri.Query()
		query.Del("n")
		uri.RawQuery = query.Encode()
		return proxies.New(uri.String()).Proxy
	}
	return http.ProxyFromEnvironment
}

type Proxies interface {

	// Name is the provider name
	Name() string

	// New a proxy
	New(endpoint string) Proxy
}

type Proxy interface {

	// Proxy is the provider implements
	Proxy(req *http.Request) (*url.URL, error)
}

type ContextDialer interface {

	// Name is the provider name
	Name() string

	// New a dialer
	New(proxy string, serverName string, dialer *net.Dialer) proxy.ContextDialer
}
