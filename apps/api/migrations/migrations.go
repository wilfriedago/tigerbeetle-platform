// Package migrations embeds the goose SQL migrations so the binary can apply them at startup
// without shipping the files separately (decision: migrations run behind a config flag).
package migrations

import "embed"

// FS holds every *.sql migration in this directory. go:embed only reaches files at or below the
// package directory, which is why this package lives next to the SQL files.
//
//go:embed *.sql
var FS embed.FS
