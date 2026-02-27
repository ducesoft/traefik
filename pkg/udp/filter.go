package udp

import (
	"cmp"
	"context"
	"fmt"
	"slices"
)

type Filter interface {
	Name() string
	Priority() int
	// Scope distinguishes global filters (0) from explicitly selected filters (1).
	Scope() int
	New(ctx context.Context, next Handler, name string) (Handler, error)
}

type NextFilter interface {
	Filter
	Next(ctx context.Context, next Handler, name string, option any) (Handler, error)
}

var filters = map[string]Filter{}

// Provide must be called during startup, before routers are built.
func Provide(filter Filter) {
	filters[filter.Name()] = filter
}

func WithFilter(name string, fn func(Filter)) {
	if filter := filters[name]; filter != nil {
		fn(filter)
	}
}

func WithNextFilter(name string, fn func(NextFilter)) {
	WithFilter(name, func(filter Filter) {
		if next, ok := filter.(NextFilter); ok {
			fn(next)
		}
	})
}

func GlobalFilters(ctx context.Context) Constructor {
	fs := make([]Filter, 0, len(filters))
	for _, filter := range filters {
		if filter.Scope() == 0 {
			fs = append(fs, filter)
		}
	}
	slices.SortFunc(fs, func(a, b Filter) int {
		return cmp.Or(cmp.Compare(b.Priority(), a.Priority()), cmp.Compare(b.Name(), a.Name()))
	})

	names := make([]string, 0, len(fs))
	for _, filter := range fs {
		names = append(names, filter.Name())
	}
	return NamedFilters(ctx, names)
}

func NamedFilters(ctx context.Context, middlewares []string) Constructor {
	constructors := make([]Constructor, 0, len(middlewares))
	for _, name := range middlewares {
		filter := filters[name]
		constructors = append(constructors, func(next Handler) (Handler, error) {
			if filter == nil {
				return nil, fmt.Errorf("UDP middleware %q does not exist", name)
			}
			var handler Handler
			var err error
			if f, ok := filter.(NextFilter); ok {
				handler, err = f.Next(ctx, next, filter.Name(), nil)
			} else {
				handler, err = filter.New(ctx, next, filter.Name())
			}
			if err != nil {
				return nil, fmt.Errorf("UDP middleware %q: %w", filter.Name(), err)
			}
			if handler == nil {
				return nil, fmt.Errorf("UDP middleware %q returned a nil handler", filter.Name())
			}
			return handler, nil
		})
	}

	return NewChain(constructors...).Then
}
