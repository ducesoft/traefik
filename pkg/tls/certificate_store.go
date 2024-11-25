package tls

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/safe"
)

// CertificateStore store for dynamic certificates.
type CertificateStore struct {
	DynamicCerts       *safe.Safe
	DefaultCertificate *tls.Certificate
	CertCache          *cache.Cache
}

// NewCertificateStore create a store for dynamic certificates.
func NewCertificateStore() *CertificateStore {
	s := &safe.Safe{}
	s.Set(make(map[string]*tls.Certificate))

	return &CertificateStore{
		DynamicCerts: s,
		CertCache:    cache.New(1*time.Hour, 10*time.Minute),
	}
}

func (c *CertificateStore) getDefaultCertificateDomains() []string {
	var allCerts []string

	if c.DefaultCertificate == nil {
		return allCerts
	}

	x509Cert, err := x509.ParseCertificate(c.DefaultCertificate.Certificate[0])
	if err != nil {
		log.Error().Err(err).Msg("Could not parse default certificate")
		return allCerts
	}

	if len(x509Cert.Subject.CommonName) > 0 {
		allCerts = append(allCerts, x509Cert.Subject.CommonName)
	}

	allCerts = append(allCerts, x509Cert.DNSNames...)

	for _, ipSan := range x509Cert.IPAddresses {
		allCerts = append(allCerts, ipSan.String())
	}

	return allCerts
}

// GetAllDomains return a slice with all the certificate domain.
func (c *CertificateStore) GetAllDomains() []string {
	allDomains := c.getDefaultCertificateDomains()

	// Get dynamic certificates
	if c.DynamicCerts != nil && c.DynamicCerts.Get() != nil {
		for domain := range c.DynamicCerts.Get().(map[string]*tls.Certificate) {
			allDomains = append(allDomains, domain)
		}
	}

	return allDomains
}

// GetBestCertificate returns the best match certificate, and caches the response.
func (c *CertificateStore) GetBestCertificate(clientHello *tls.ClientHelloInfo) *tls.Certificate {
	if c == nil {
		return nil
	}
	serverName := strings.ToLower(strings.TrimSpace(clientHello.ServerName))
	if len(serverName) == 0 {
		// If no ServerName is provided, Check for local IP address matches
		host, _, err := net.SplitHostPort(clientHello.Conn.LocalAddr().String())
		if err != nil {
			log.Debug().Err(err).Msg("Could not split host/port")
		}
		serverName = strings.TrimSpace(host)
	}

	if cert, ok := c.CertCache.Get(serverName); ok {
		return cert.(*tls.Certificate)
	}

	matchedCerts := map[string]*tls.Certificate{}
	if c.DynamicCerts != nil && c.DynamicCerts.Get() != nil {
		for domains, cert := range c.DynamicCerts.Get().(map[string]*tls.Certificate) {
			for _, certDomain := range strings.Split(domains, ",") {
				if matchDomain(serverName, certDomain) {
					matchedCerts[certDomain] = cert
				}
			}
		}
	}

	if len(matchedCerts) > 0 {
		// sort map by keys
		keys := make([]string, 0, len(matchedCerts))
		for k := range matchedCerts {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// cache best match
		c.CertCache.SetDefault(serverName, matchedCerts[keys[len(keys)-1]])

		log.Debug().Msgf("certificate matched: %s for server name: %s", strings.Join(keys, ","), serverName)
		return matchedCerts[keys[len(keys)-1]]
	}

	certs := [3][]*tls.Certificate{}
	if c.DynamicCerts != nil && c.DynamicCerts.Get() != nil {
		for domains, cert := range c.DynamicCerts.Get().(map[string]*tls.Certificate) {
			for _, certDomain := range strings.Split(domains, ",") {
				if domain := c.hostnameInSNI(certDomain); "" != domain {
					certs[0] = append(certs[0], cert)
				}
				if "127.0.0.1" != certDomain {
					certs[1] = append(certs[1], cert)
					continue
				}
				certs[2] = append(certs[2], cert)
			}
		}
	}
	for _, crts := range certs {
		if len(crts) > 0 {
			log.Debug().Msgf("certificate backoff matched for server name: %s", serverName)
			return crts[0]
		}
	}

	log.Debug().Msgf("no matching certificate found for server name: %s", serverName)
	return nil
}

func (c *CertificateStore) hostnameInSNI(name string) string {
	host := name
	if len(host) > 0 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	if i := strings.LastIndex(host, "%"); i > 0 {
		host = host[:i]
	}
	if net.ParseIP(host) != nil {
		return ""
	}
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	return name
}

// GetCertificate returns the first certificate matching all the given domains.
func (c *CertificateStore) GetCertificate(domains []string) *tls.Certificate {
	if c == nil {
		return nil
	}

	sort.Strings(domains)
	domainsKey := strings.Join(domains, ",")

	if cert, ok := c.CertCache.Get(domainsKey); ok {
		return cert.(*tls.Certificate)
	}

	if c.DynamicCerts != nil && c.DynamicCerts.Get() != nil {
		for certDomains, cert := range c.DynamicCerts.Get().(map[string]*tls.Certificate) {
			if domainsKey == certDomains {
				c.CertCache.SetDefault(domainsKey, cert)
				return cert
			}

			var matchedDomains []string
			for _, certDomain := range strings.Split(certDomains, ",") {
				for _, checkDomain := range domains {
					if certDomain == checkDomain {
						matchedDomains = append(matchedDomains, certDomain)
					}
				}
			}

			if len(matchedDomains) == len(domains) {
				c.CertCache.SetDefault(domainsKey, cert)
				return cert
			}
		}
	}

	return nil
}

// ResetCache clears the cache in the store.
func (c *CertificateStore) ResetCache() {
	if c.CertCache != nil {
		c.CertCache.Flush()
	}
}

// matchDomain returns whether the server name matches the cert domain.
// The server name, from TLS SNI, must not have trailing dots (https://datatracker.ietf.org/doc/html/rfc6066#section-3).
// This is enforced by https://github.com/golang/go/blob/d3d7998756c33f69706488cade1cd2b9b10a4c7f/src/crypto/tls/handshake_messages.go#L423-L427.
func matchDomain(serverName, certDomain string) bool {
	// TODO: assert equality after removing the trailing dots?
	if serverName == certDomain {
		return true
	}

	for len(certDomain) > 0 && certDomain[len(certDomain)-1] == '.' {
		certDomain = certDomain[:len(certDomain)-1]
	}

	labels := strings.Split(serverName, ".")
	for i := range labels {
		labels[i] = "*"
		candidate := strings.Join(labels, ".")
		if certDomain == candidate {
			return true
		}
	}
	return false
}
