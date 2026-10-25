package rpc

import (
	"context"
	"slices"
	"strings"
)

// Handler answers one method: data for a reply, or an error mapped to a problem.
type Handler func(ctx context.Context, req Request) (any, error)

// Router maps method names to handlers.
type Router struct {
	routes map[string]Handler
}

// NewRouter returns an empty router.
func NewRouter() *Router {
	return &Router{routes: map[string]Handler{}}
}

// Handle registers a method. Methods are "v1.<operationId>"; registering one twice is a bug and
// panics at start-up.
func (r *Router) Handle(method string, h Handler) {
	if !strings.HasPrefix(method, "v1.") {
		panic("rpc: method " + method + " does not start with v1.")
	}
	if _, ok := r.routes[method]; ok {
		panic("rpc: method " + method + " registered twice")
	}
	r.routes[method] = h
}

// Methods lists the registered methods in order.
func (r *Router) Methods() []string {
	out := make([]string, 0, len(r.routes))
	for m := range r.routes {
		out = append(out, m)
	}
	slices.Sort(out)
	return out
}

func (r *Router) lookup(method string) (Handler, bool) {
	h, ok := r.routes[method]
	return h, ok
}
