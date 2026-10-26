package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func TestBuild(t *testing.T) {
	t.Parallel()
	first, err := build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	second, err := build()

	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("the schema is not byte-stable")
	}

	if !strings.HasPrefix(string(first), header) {
		t.Fatal("missing header")
	}

	if _, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphql", Input: string(first)}); err != nil {
		t.Fatalf("gqlparser rejects the schema: %v", err)
	}
}
