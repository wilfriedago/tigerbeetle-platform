package bootstrap

import "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/app"

// Option adjusts the build. Only tests use options.
type Option func(*options)

type options struct {
	wrapLedgerStore func(app.LedgerStore) app.LedgerStore
}

func newOptions(opts []Option) options {
	o := options{wrapLedgerStore: func(s app.LedgerStore) app.LedgerStore { return s }}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithLedgerStore wraps the TigerBeetle store the use cases call, so a test can inject faults
// between the API and the cluster, such as the crash windows of ADR-0007.
func WithLedgerStore(wrap func(app.LedgerStore) app.LedgerStore) Option {
	return func(o *options) { o.wrapLedgerStore = wrap }
}
