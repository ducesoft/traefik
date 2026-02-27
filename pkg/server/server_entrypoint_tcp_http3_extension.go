package server

import (
	"net"
	"net/http"

	"github.com/quic-go/quic-go/http3"
	"github.com/rs/zerolog/log"
)

// HTTP3ServerExtension owns the HTTP/3 listener lifecycle after configuring the server.
type HTTP3ServerExtension interface {
	Serve(net.PacketConn) error
	Close() error
	SetQUICHeaders(http.Header, net.PacketConn) error
}

type compositeHTTP3ServerExtension struct {
	hooks []HTTP3ServerExtension
}

func (that *compositeHTTP3ServerExtension) Serve(conn net.PacketConn) error {
	for _, hook := range that.hooks {
		if err := hook.Serve(conn); nil != err {
			return err
		}
	}
	return nil
}

func (that *compositeHTTP3ServerExtension) Close() error {
	for _, hook := range that.hooks {
		if err := hook.Close(); nil != err {
			log.Error().Err(err)
		}
	}
	return nil
}

func (that *compositeHTTP3ServerExtension) SetQUICHeaders(header http.Header, conn net.PacketConn) error {
	for _, hook := range that.hooks {
		if err := hook.SetQUICHeaders(header, conn); nil != err {
			return err
		}
	}
	return nil
}

// HTTP3ServerConfigurer configures an HTTP/3 server and optionally takes over its listener lifecycle.
type HTTP3ServerConfigurer func(server *http3.Server) HTTP3ServerExtension

var http3ServerExtensionRegistry []HTTP3ServerConfigurer

// RegisterHTTP3ServerConfigurer registers the HTTP/3 server extension configurer.
func RegisterHTTP3ServerConfigurer(configurer HTTP3ServerConfigurer) {
	http3ServerExtensionRegistry = append(http3ServerExtensionRegistry, configurer)
}

func configureHTTP3ServerExtension(server *http3.Server) HTTP3ServerExtension {
	hooks := make([]HTTP3ServerExtension, len(http3ServerExtensionRegistry))
	for idx, configurer := range http3ServerExtensionRegistry {
		hooks[idx] = configurer(server)
	}
	if 1 == len(hooks) {
		return hooks[0]
	}
	return &compositeHTTP3ServerExtension{hooks: hooks}
}
