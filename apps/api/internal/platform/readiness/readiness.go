// Package readiness probes the dependencies of the process concurrently, each with its own
// timeout, so a slow dependency does not hide the state of the others. It answers /readyz and
// /v1/_info (plan section 13.2).
package readiness

import (
	"context"
	"sync"
	"time"
)

// Probe checks one dependency and returns nil when it answered in time.
type Probe func(ctx context.Context) error

// Result is the outcome of one probe.
type Result struct {
	Name string
	// Required dependencies decide readiness; the others are only reported.
	Required bool
	// Err is nil when the dependency answered in time. It may wrap context.DeadlineExceeded.
	Err     error
	Latency time.Duration
}

// Up reports whether the dependency answered.
func (r Result) Up() bool { return r.Err == nil }

// Report holds the result of every probe, in the order the probes were added.
type Report []Result

// Degraded is true when every required dependency is up but an optional one is down: the
// process serves, without one of its transports (plan section 13.2).
func (r Report) Degraded() bool {
	if !r.Ready() {
		return false
	}
	for _, res := range r {
		if !res.Required && !res.Up() {
			return true
		}
	}
	return false
}

// Ready is true when every required dependency is up.
func (r Report) Ready() bool {
	for _, res := range r {
		if res.Required && !res.Up() {
			return false
		}
	}
	return true
}

// Result returns the result of the named probe.
func (r Report) Result(name string) (Result, bool) {
	for _, res := range r {
		if res.Name == name {
			return res, true
		}
	}
	return Result{}, false
}

type probe struct {
	name     string
	required bool
	check    Probe
}

// Checker holds the probes. Add them while wiring, before the first Check.
type Checker struct {
	timeout time.Duration
	probes  []probe
}

// New returns a checker whose probes each get timeout to answer.
func New(timeout time.Duration) *Checker {
	return &Checker{timeout: timeout}
}

// Require adds a dependency the process cannot serve without, such as Postgres.
func (c *Checker) Require(name string, check Probe) {
	c.probes = append(c.probes, probe{name: name, required: true, check: check})
}

// Optional adds a dependency the process serves without, such as a broker: it is reported, and
// its outage makes the process degraded, not unready.
func (c *Checker) Optional(name string, check Probe) {
	c.probes = append(c.probes, probe{name: name, required: false, check: check})
}

// Timeout is the time each probe gets to answer.
func (c *Checker) Timeout() time.Duration {
	return c.timeout
}

// Check runs every probe concurrently and waits for all of them.
func (c *Checker) Check(ctx context.Context) Report {
	report := make(Report, len(c.probes))
	var wg sync.WaitGroup
	for i, p := range c.probes {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()
			start := time.Now()
			err := p.check(ctx)
			report[i] = Result{Name: p.name, Required: p.required, Err: err, Latency: time.Since(start)}
		})
	}
	wg.Wait()
	return report
}
