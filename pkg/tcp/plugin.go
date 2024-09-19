package tcp

import (
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"golang.org/x/net/proxy"
	"net/url"
)

const DefaultName = "default"

var (
	netsDialer = map[string]WriteCloserDialer{}
)

func ProvideDialer(dialer WriteCloserDialer) {
	netsDialer[dialer.Name()] = dialer
}

func CreateDialer(tcp *dynamic.TCPServersTransport, dialer proxy.Dialer) proxy.Dialer {
	serverName := func(tls *dynamic.TLSClientConfig) string {
		if nil == tls || "" == tls.ServerName {
			return ""
		}
		return tls.ServerName
	}(tcp.TLS)
	d := func(tls *dynamic.TLSClientConfig) proxy.Dialer {
		if nil == tls || "" == tls.ServerName {
			return dialer
		}
		if netDialer, ok := netsDialer[tls.ServerName]; ok && nil != netDialer {
			return netDialer.New(tcp.Proxy, tls.ServerName, dialer)
		}
		if netDialer, ok := netsDialer[DefaultName]; ok && nil != netDialer {
			return netDialer.New(tcp.Proxy, tls.ServerName, dialer)
		}
		return dialer
	}(tcp.TLS)
	if "" == tcp.Proxy {
		return proxy.FromEnvironmentUsing(d)
	}
	uri, err := url.Parse(tcp.Proxy)
	if nil != err {
		log.Error().Msgf("Error while create transport proxy, %v", err)
		return proxy.FromEnvironmentUsing(d)
	}
	if netDialer, ok := netsDialer[uri.Scheme]; ok && nil != netDialer {
		return netDialer.New(tcp.Proxy, serverName, dialer)
	}
	socks5, err := proxy.FromURL(uri, d)
	if nil != err {
		log.Error().Msgf("Error while create transport proxy, %v", err)
		return proxy.FromEnvironmentUsing(d)
	}
	return socks5
}

type WriteCloserDialer interface {

	// Name is the provider name
	Name() string

	// New a dialer
	New(proxy string, serverName string, dialer proxy.Dialer) proxy.Dialer
}
