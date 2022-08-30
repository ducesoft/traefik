/*
 * Copyright (c) 2000, 2099, trustbe and/or its affiliates. All rights reserved.
 * TRUSTBE PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package service

import (
	"net/http"
	"net/url"
)

var httpProxies map[string]HTTPProxy

func ProvideHTTPProxy(proxy HTTPProxy) {
	httpProxies[proxy.Name()] = proxy
}

func LoadHTTPProxy(name string) func(req *http.Request) (*url.URL, error) {
	if proxy, ok := httpProxies[name]; ok && nil != proxy {
		return proxy.Proxy
	}
	return http.ProxyFromEnvironment
}

type HTTPProxy interface {

	// Name is the provider name
	Name() string

	// Proxy is the provider implements
	Proxy(req *http.Request) (*url.URL, error)
}
