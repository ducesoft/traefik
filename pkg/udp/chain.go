package udp

import (
	"errors"
	"slices"
)

type Constructor func(Handler) (Handler, error)

type Chain struct {
	constructors []Constructor
}

func NewChain(constructors ...Constructor) Chain {
	return Chain{constructors: slices.Clone(constructors)}
}

func (c Chain) Then(h Handler) (Handler, error) {
	if h == nil {
		return nil, errors.New("cannot add a nil handler to the chain")
	}

	// Build from the tail so requests visit middleware in declaration order.
	for i := len(c.constructors) - 1; i >= 0; i-- {
		if c.constructors[i] == nil {
			return nil, errors.New("cannot use a nil middleware constructor")
		}
		handler, err := c.constructors[i](h)
		if err != nil {
			return nil, err
		}
		if handler == nil {
			return nil, errors.New("middleware returned a nil handler")
		}
		h = handler
	}

	return h, nil
}

func (c Chain) Append(constructors ...Constructor) Chain {
	newCons := make([]Constructor, 0, len(c.constructors)+len(constructors))
	newCons = append(newCons, c.constructors...)
	newCons = append(newCons, constructors...)
	return Chain{constructors: newCons}
}

func (c Chain) Extend(chain Chain) Chain {
	return c.Append(chain.constructors...)
}
