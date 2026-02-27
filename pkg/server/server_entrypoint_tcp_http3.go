package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/static"
	tcprouter "github.com/traefik/traefik/v3/pkg/server/router/tcp"
)

type http3server struct {
	*http3.Server

	http3conn net.PacketConn
	extension HTTP3ServerExtension

	lock   sync.RWMutex
	getter func(info *tls.ClientHelloInfo) (*tls.Config, error)
}

func newHTTP3Server(ctx context.Context, name string, config *static.EntryPoint, httpsServer *httpServer) (*http3server, error) {
	var conn net.PacketConn
	var err error

	if config.HTTP3 == nil {
		return nil, nil
	}

	if config.HTTP3.AdvertisedPort < 0 {
		return nil, errors.New("advertised port must be greater than or equal to zero")
	}

	// if we have predefined connections from socket activation
	if socketActivation.isEnabled() {
		conn, err = socketActivation.getConn(name)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Str("name", name).Msg("Unable to use socket activation for entrypoint")
		}
	}

	if conn == nil {
		listenConfig := newListenConfig(config)
		conn, err = listenConfig.ListenPacket(ctx, "udp", config.GetAddress())
		if err != nil {
			return nil, fmt.Errorf("starting listener: %w", err)
		}
	}

	h3 := &http3server{
		http3conn: conn,
		getter: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			return nil, errors.New("no tls config")
		},
	}

	h3.Server = &http3.Server{
		Addr:            config.GetAddress(),
		Port:            config.HTTP3.AdvertisedPort,
		Handler:         httpsServer.Server.(*http.Server).Handler,
		TLSConfig:       &tls.Config{GetConfigForClient: h3.getGetConfigForClient},
		EnableDatagrams: config.HTTP3.EnableDatagrams,
		QUICConfig:      buildQUICConfig(config.HTTP3),
	}
	h3.extension = configureHTTP3ServerExtension(h3.Server)

	previousHandler := httpsServer.Server.(*http.Server).Handler

	httpsServer.Server.(*http.Server).Handler = http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := h3.setQUICHeaders(rw.Header()); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Failed to set HTTP3 headers")
		}

		previousHandler.ServeHTTP(rw, req)
	})

	return h3, nil
}

func (e *http3server) setQUICHeaders(header http.Header) error {
	if nil != e.extension {
		return e.extension.SetQUICHeaders(header, e.http3conn)
	}
	return e.Server.SetQUICHeaders(header)
}

func buildQUICConfig(config *static.HTTP3Config) *quic.Config {
	versions := make([]quic.Version, len(config.Versions))
	for index, version := range config.Versions {
		versions[index] = quic.Version(version)
	}
	return &quic.Config{
		Versions:                         versions,
		HandshakeIdleTimeout:             time.Duration(config.HandshakeIdleTimeout),
		MaxIdleTimeout:                   time.Duration(config.MaxIdleTimeout),
		InitialStreamReceiveWindow:       config.InitialStreamReceiveWindow,
		MaxStreamReceiveWindow:           config.MaxStreamReceiveWindow,
		InitialConnectionReceiveWindow:   config.InitialConnectionReceiveWindow,
		MaxConnectionReceiveWindow:       config.MaxConnectionReceiveWindow,
		MaxIncomingStreams:               config.MaxIncomingStreams,
		MaxIncomingUniStreams:            config.MaxIncomingUniStreams,
		KeepAlivePeriod:                  time.Duration(config.KeepAlivePeriod),
		InitialPacketSize:                config.InitialPacketSize,
		DisablePathMTUDiscovery:          config.DisablePathMTUDiscovery,
		Allow0RTT:                        config.Allow0RTT,
		EnableDatagrams:                  config.EnableDatagrams,
		EnableStreamResetPartialDelivery: config.EnableStreamResetPartialDelivery,
	}
}

func (e *http3server) Start() error {
	if e.extension != nil {
		return e.extension.Serve(e.http3conn)
	}
	return e.Serve(e.http3conn)
}

func (e *http3server) Switch(rt *tcprouter.Router) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.getter = rt.GetTLSGetClientInfo()
}

func (e *http3server) Shutdown(_ context.Context) error {
	if e.extension != nil {
		return e.extension.Close()
	}
	// TODO: use e.Server.CloseGracefully() when available.
	return e.Server.Close()
}

func (e *http3server) getGetConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.getter(info)
}
