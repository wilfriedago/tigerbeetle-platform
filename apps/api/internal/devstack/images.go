// Package devstack exposes the container images pinned in deploy/compose/compose.yaml, so the
// integration tests and the local stack always run the same TigerBeetle, Postgres, RabbitMQ and
// NATS versions.
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
	RabbitMQ    string
	NATS        string
}

var (
	tigerbeetleImage = regexp.MustCompile(`(?m)^\s*image:\s*(ghcr\.io/tigerbeetle/tigerbeetle:\S+)`)
	postgresImage    = regexp.MustCompile(`(?m)^\s*image:\s*(postgres:\S+)`)
	rabbitmqImage    = regexp.MustCompile(`(?m)^\s*image:\s*(rabbitmq:\S+)`)
	natsImage        = regexp.MustCompile(`(?m)^\s*image:\s*(nats:\S+)`)
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
	mq := rabbitmqImage.FindSubmatch(data)
	nats := natsImage.FindSubmatch(data)
	if tb == nil || pg == nil || mq == nil || nats == nil {
		return Images{}, fmt.Errorf("devstack: %s does not pin the tigerbeetle, postgres, rabbitmq and nats images", path)
	}
	return Images{TigerBeetle: string(tb[1]), Postgres: string(pg[1]), RabbitMQ: string(mq[1]), NATS: string(nats[1])}, nil
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
