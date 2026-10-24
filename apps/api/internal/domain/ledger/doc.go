// Package ledger holds the pure domain of the platform: ledgers (namespaces), assets, addresses,
// identifiers and the errors use cases return. It has no I/O and imports nothing from internal,
// which the depguard rules in .golangci.yml enforce.
package ledger
