// Package devstack exposes the container images pinned in deploy/compose/compose.yaml, so the
// integration tests and the local stack always run the same TigerBeetle and Postgres versions.
package devstack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Images are the pinned image references.
type Images struct {
	TigerBeetle string
	Postgres    string
}

var (
	tigerbeetleImage = regexp.MustCompile(`(?m)^\s*image:\s*(ghcr\.io/tigerbeetle/tigerbeetle:\S+)`)
	postgresImage    = regexp.MustCompile(`(?m)^\s*image:\s*(postgres:\S+)`)
)

// Load finds the repository root from the current directory and parses the compose file.
func Load() (Images, error) {
	root, err := repoRoot()
	if err != nil {
		return Images{}, err
	}
	return Parse(filepath.Join(root, "deploy", "compose", "compose.yaml"))
}

// Parse reads the image references from a compose file.
func Parse(path string) (Images, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is built from the repository root
	if err != nil {
		return Images{}, fmt.Errorf("devstack: %w", err)
	}
	tb := tigerbeetleImage.FindSubmatch(data)
	pg := postgresImage.FindSubmatch(data)
	if tb == nil || pg == nil {
		return Images{}, fmt.Errorf("devstack: %s does not pin both the tigerbeetle and postgres images", path)
	}
	return Images{TigerBeetle: string(tb[1]), Postgres: string(pg[1])}, nil
}

// repoRoot walks up from the working directory until it finds pnpm-workspace.yaml.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "pnpm-workspace.yaml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("devstack: repository root (pnpm-workspace.yaml) not found")
		}
		dir = parent
	}
}
