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
	d := func(tls *dynamic.TLSClientConfig) proxy.Dialer {
		if nil == tls || "" == tls.ServerName {
			return dialer
		}
		if netDialer, ok := netsDialer[tls.ServerName]; ok && nil != netDialer {
			return netDialer.New(tls.ServerName, dialer)
		}
		if netDialer, ok := netsDialer[DefaultName]; ok && nil != netDialer {
			return netDialer.New(tls.ServerName, dialer)
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
	New(serverName string, dialer proxy.Dialer) proxy.Dialer
}
