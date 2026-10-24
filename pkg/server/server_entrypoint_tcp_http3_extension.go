package server

import (
	"net"
	"net/http"

	"github.com/quic-go/quic-go/http3"
	"github.com/rs/zerolog/log"
)

// HTTP3Server owns the HTTP/3 listener lifecycle after configuring the server.
type HTTP3Server interface {
	Serve(net.PacketConn) (bool, error)
	Close() error
	SetQUICHeaders(http.Header, net.PacketConn) (bool, error)
}

type compositeHTTP3ServerExtension struct {
	server *http3.Server
	hooks  []HTTP3Server
}

func (that *compositeHTTP3ServerExtension) Serve(conn net.PacketConn) (bool, error) {
	for _, hook := range that.hooks {
		match, err := hook.Serve(conn)
		if nil != err {
			return false, err
		}
		if match {
			return match, nil
		}
	}
	return true, that.server.Serve(conn)
}

func (that *compositeHTTP3ServerExtension) Close() error {
	for _, hook := range that.hooks {
		if err := hook.Close(); nil != err {
			log.Error().Err(err)
		}
	}
	return that.server.Close()
}

func (that *compositeHTTP3ServerExtension) SetQUICHeaders(header http.Header, conn net.PacketConn) (bool, error) {
	for _, hook := range that.hooks {
		match, err := hook.SetQUICHeaders(header, conn)
		if nil != err {
			return false, err
		}
		if match {
			return match, nil
		}
	}
	return true, that.server.SetQUICHeaders(header)
}

// HTTP3ServerConfigurer configures an HTTP/3 server and optionally takes over its listener lifecycle.
type HTTP3ServerConfigurer func(server *http3.Server) HTTP3Server

var http3ServerExtensionRegistry []HTTP3ServerConfigurer

// RegisterHTTP3ServerConfigurer registers the HTTP/3 server extension configurer.
func RegisterHTTP3ServerConfigurer(configurer HTTP3ServerConfigurer) {
	http3ServerExtensionRegistry = append(http3ServerExtensionRegistry, configurer)
}

func configureHTTP3ServerExtension(server *http3.Server) HTTP3Server {
	hooks := make([]HTTP3Server, len(http3ServerExtensionRegistry))
	for idx, configurer := range http3ServerExtensionRegistry {
		hooks[idx] = configurer(server)
	}
	return &compositeHTTP3ServerExtension{
		server: server,
		hooks:  hooks,
	}
}
