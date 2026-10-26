package migrations_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// tenantTables are the tables whose rows belong to one organization (ADR-0027). Every query that
// touches one must filter or write by organization_id, so no query can read or change another
// tenant's rows.
var tenantTables = []string{
	"ledgers", "ledger_assets", "account_rules", "event_types", "addresses", "accounts",
	"write_requests", "transactions", "holds", "grants", "api_keys", "machine_clients",
}

// TestQueriesNameTheOrganization reads every sqlc query of every module. A query that touches a
// tenant table must mention organization_id, or carry a "-- tenant-free:" comment that says why it
// may read across organizations.
func TestQueriesNameTheOrganization(t *testing.T) {
	files, err := filepath.Glob("../internal/modules/*/infrastructure/postgres/queries/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no query files found")
	}
	touches := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE)\s+(` + strings.Join(tenantTables, "|") + `)\b`)
	name := regexp.MustCompile(`-- name: (\w+)`)
	checked := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, query := range strings.Split(string(raw), "-- name: ")[1:] {
			query = "-- name: " + query
			if !touches.MatchString(query) || strings.Contains(query, "-- tenant-free:") {
				continue
			}
			checked++
			if !strings.Contains(query, "organization_id") {
				t.Errorf("%s: query %s touches a tenant table without organization_id", filepath.Base(file), name.FindStringSubmatch(query)[1])
			}
		}
	}
	if checked < 40 {
		t.Errorf("checked %d queries, expected at least 40: has the layout changed?", checked)
	}
}
