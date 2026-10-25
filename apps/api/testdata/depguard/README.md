# depguard probe fixtures

Each `bad.go` here breaks exactly one depguard rule of `apps/api/.golangci.yml`, named on its
`// probe:` line, from a path that mirrors the real tree. `just lint-depguard-probe` lints each
fixture package with depguard alone and fails unless it reports exactly that rule once. The
fixtures compile, and `./...` skips `testdata`, so they never reach the build or the normal lint.

Add a fixture when you add a rule or a module.
