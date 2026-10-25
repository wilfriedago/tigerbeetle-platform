// Package ledger is the shared kernel of the modules (docs/plans/ddd-transports/plan.md, section
// 4.4): the types and rules two or more modules use, such as ledger names, assets, addresses,
// identifiers, amounts, account rules, event types and the errors use cases return. It has no I/O
// and imports nothing from internal, which the depguard rules in .golangci.yml enforce.
package ledger
