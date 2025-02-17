/*
 * Copyright (c) 2019, 2023, ducesoft and/or its affiliates. All rights reserved.
 * DUCESOFT PROPRIETARY/CONFIDENTIAL. Use is subject to license terms.
 *
 *
 */

package middleware

import (
	"context"
	"github.com/containous/alice"
	"net/http"
	"sort"
)

var filters = map[string]Filter{}

type Filter interface {

	// Name is middleware name
	Name() string

	// Priority more than has more priority
	Priority() int

	// Scope is middleware effect scope, 0 is global, others is customized.
	Scope() int

	// New middleware instance
	New(ctx context.Context, next http.Handler, name string) (http.Handler, error)
}

// Provide the middleware
func Provide(filter Filter) {
	filters[filter.Name()] = filter
}

func WithFilter(name string, fn func(m Filter)) {
	if x := filters[name]; nil != x {
		fn(x)
	}
}

func GlobalFilters(ctx context.Context) alice.Constructor {
	var fs []Filter
	for _, filter := range filters {
		if filter.Scope() == 0 {
			fs = append(fs, filter)
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Priority() < fs[j].Priority() })
	constructor := func(next http.Handler) (http.Handler, error) {
		var err error
		for _, filter := range fs {
			if next, err = filter.New(ctx, next, filter.Name()); nil != err {
				return nil, err
			}
		}
		return next, nil
	}
	return constructor
}
