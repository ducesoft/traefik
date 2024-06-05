package tcp

import (
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"golang.org/x/net/proxy"
)

const DefaultName = "default"

var (
	netsDialer = map[string]WriteCloserDialer{}
)

func ProvideDialer(dialer WriteCloserDialer) {
	netsDialer[dialer.Name()] = dialer
}

func CreateDialer(tls *dynamic.TLSClientConfig, dialer proxy.Dialer) proxy.Dialer {
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
}

type WriteCloserDialer interface {

	// Name is the provider name
	Name() string

	// New a dialer
	New(serverName string, dialer proxy.Dialer) proxy.Dialer
}
