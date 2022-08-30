/*
 * Copyright (c) 2000, 2099, trustbe and/or its affiliates. All rights reserved.
 * TRUSTBE PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package service

import (
	"github.com/traefik/traefik/v2/pkg/log"
	"net/http"
	"net/url"
)

var httpProxies map[string]Proxies

func ProvideProxy(proxies Proxies) {
	httpProxies[proxies.Name()] = proxies
}

func CreateProxy(endpoint string) func(req *http.Request) (*url.URL, error) {
	if "" == endpoint {
		return http.ProxyFromEnvironment
	}
	uri, err := url.Parse(endpoint)
	if nil != err {
		log.WithoutContext().Error("Error while create transport proxy", err)
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
