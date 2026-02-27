package udpmiddleware

import (
	"context"
	"fmt"

	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/server/provider"
	"github.com/traefik/traefik/v3/pkg/udp"
)

type Builder struct {
	configs map[string]*dynamic.UDPMiddleware
}

func NewBuilder(configs map[string]*dynamic.UDPMiddleware) *Builder {
	return &Builder{configs: configs}
}

func (b *Builder) BuildChain(ctx context.Context, middlewares []string) udp.Chain {
	chain := udp.NewChain()
	for _, name := range middlewares {
		middlewareName := provider.GetQualifiedName(ctx, name)
		config, configured := b.configs[middlewareName]
		if !configured {
			// Preserve direct registration names for filters that need no configuration.
			chain = chain.Append(udp.NamedFilters(ctx, []string{name}))
			continue
		}
		chain = chain.Append(func(next udp.Handler) (udp.Handler, error) {
			if config == nil || len(config.Anyone) != 1 {
				return nil, fmt.Errorf("UDP middleware %q must configure exactly one filter in anyone", middlewareName)
			}
			for filterName, option := range config.Anyone {
				var filter udp.NextFilter
				udp.WithNextFilter(filterName, func(f udp.NextFilter) { filter = f })
				if filter == nil {
					return nil, fmt.Errorf("UDP middleware %q: NextFilter %q does not exist", middlewareName, filterName)
				}
				ctx := provider.AddInContext(ctx, middlewareName)
				handler, err := filter.Next(ctx, next, middlewareName, option)
				if err != nil {
					return nil, fmt.Errorf("UDP middleware %q: %w", middlewareName, err)
				}
				if handler == nil {
					return nil, fmt.Errorf("UDP middleware %q returned a nil handler", middlewareName)
				}
				return handler, nil
			}
			return next, nil
		})
	}
	return chain
}
