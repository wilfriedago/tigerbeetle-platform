// Package directorytest is the access module's Directory in memory, for the tests of the module
// and of the API: Keycloak's organizations, accounts, memberships and invitations without a
// Keycloak.
package directorytest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

var _ domain.Directory = (*Directory)(nil)

// Directory is Keycloak in memory. It records the emails it would send. It is not safe for
// concurrent use beyond what a lock around each call gives; every method takes the lock.
type Directory struct {
	mu          sync.Mutex
	orgs        map[string]string          // alias to ID
	people      map[string]*domain.Person  // subject to person
	members     map[string]map[string]bool // org ID to subjects
	invitations map[string][]domain.Invitation
	emails      []string // "setup <subject>", "invite <subject>", "resend <invitation>"
	next        int
	issuer      string
}

// New returns an empty directory whose people's tokens come from the issuer.
func New(issuer string) *Directory {
	return &Directory{
		issuer: issuer,
		orgs:   map[string]string{}, people: map[string]*domain.Person{},
		members: map[string]map[string]bool{}, invitations: map[string][]domain.Invitation{},
	}
}

// AddPerson adds an account that can sign in.
func (d *Directory) AddPerson(subject, email string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.people[subject] = &domain.Person{Subject: subject, Email: email, FirstName: strings.ToUpper(subject[:1]) + subject[1:]}
}

// AddOrganization adds an organization and returns its ID.
func (d *Directory) AddOrganization(alias string) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.orgs[alias] = d.newID()
	return d.orgs[alias]
}

// newID returns a new ID shaped like Keycloak's, a UUID, which the platform stores as one.
func (d *Directory) newID() string {
	d.next++
	return fmt.Sprintf("00000000-0000-4000-8000-%012x", d.next)
}

// AddMember makes the person a member of the organization with the alias.
func (d *Directory) AddMember(_ context.Context, orgID, subject string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.members[orgID][subject] {
		return ledger.ErrAlreadyExists
	}
	d.member(orgID, subject)
	return nil
}

// IsMember reports whether the person is a member of the organization with the ID.
func (d *Directory) IsMember(orgID, subject string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.members[orgID][subject]
}

// Person returns the account with the subject, false when there is none.
func (d *Directory) Person(subject string) (domain.Person, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, ok := d.people[subject]
	if !ok {
		return domain.Person{}, false
	}
	return *p, true
}

// People counts the accounts.
func (d *Directory) People() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.people)
}

// PendingInvitations returns the invitations of the organization with the ID.
func (d *Directory) PendingInvitations(orgID string) []domain.Invitation {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.invitations[orgID])
}

// Emails returns what the directory would have sent, in order.
func (d *Directory) Emails() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.emails)
}

func (d *Directory) member(orgID, subject string) {
	if d.members[orgID] == nil {
		d.members[orgID] = map[string]bool{}
	}
	d.members[orgID][subject] = true
}

// Issuer implements domain.Directory.
func (d *Directory) Issuer() string { return d.issuer }

// OrganizationID implements domain.Directory.
func (d *Directory) OrganizationID(_ context.Context, alias string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id, ok := d.orgs[alias]
	if !ok {
		return "", ledger.ErrNotFound
	}
	return id, nil
}

// CreateOrganization implements domain.Directory.
func (d *Directory) CreateOrganization(_ context.Context, alias, _ string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.orgs[alias]; ok {
		return "", ledger.ErrAlreadyExists
	}
	d.orgs[alias] = d.newID()
	return d.orgs[alias], nil
}

// RenameOrganization accepts any name.
func (d *Directory) RenameOrganization(context.Context, string, string, string) error { return nil }

// PersonByEmail implements domain.Directory.
func (d *Directory) PersonByEmail(_ context.Context, email string) (domain.Person, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range d.people {
		if strings.EqualFold(p.Email, email) {
			return *p, nil
		}
	}
	return domain.Person{}, ledger.ErrNotFound
}

// CreatePerson implements domain.Directory.
func (d *Directory) CreatePerson(_ context.Context, email, first, last string) (domain.Person, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p := &domain.Person{Subject: d.newID(), Email: email, FirstName: first, LastName: last, SetupPending: true}
	d.people[p.Subject] = p
	return *p, nil
}

// SendAccountSetup implements domain.Directory.
func (d *Directory) SendAccountSetup(_ context.Context, subject string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.emails = append(d.emails, "setup "+subject)
	return nil
}

// Members implements domain.Directory.
func (d *Directory) Members(_ context.Context, orgID string) ([]domain.Person, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []domain.Person
	for s := range d.members[orgID] {
		out = append(out, *d.people[s])
	}
	return out, nil
}

// Member implements domain.Directory.
func (d *Directory) Member(_ context.Context, orgID, subject string) (domain.Person, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.members[orgID][subject] {
		return domain.Person{}, ledger.ErrNotFound
	}
	return *d.people[subject], nil
}

// RemoveMember implements domain.Directory.
func (d *Directory) RemoveMember(_ context.Context, orgID, subject string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.members[orgID][subject] {
		return ledger.ErrNotFound
	}
	delete(d.members[orgID], subject)
	return nil
}

// Invite implements domain.Directory.
func (d *Directory) Invite(_ context.Context, orgID, subject string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.emails = append(d.emails, "invite "+subject)
	sent := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	d.invitations[orgID] = append(d.invitations[orgID], domain.Invitation{
		ID: "inv-" + subject, Email: d.people[subject].Email, SentAt: sent, ExpiresAt: sent.Add(12 * time.Hour),
	})
	return nil
}

// Invitations implements domain.Directory.
func (d *Directory) Invitations(_ context.Context, orgID string) ([]domain.Invitation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.invitations[orgID], nil
}

// ResendInvitation implements domain.Directory.
func (d *Directory) ResendInvitation(_ context.Context, orgID, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, inv := range d.invitations[orgID] {
		if inv.ID == id {
			d.emails = append(d.emails, "resend "+id)
			return nil
		}
	}
	return ledger.ErrNotFound
}

// DeleteInvitation implements domain.Directory.
func (d *Directory) DeleteInvitation(_ context.Context, orgID, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	before := len(d.invitations[orgID])
	d.invitations[orgID] = slices.DeleteFunc(d.invitations[orgID], func(i domain.Invitation) bool { return i.ID == id })
	if len(d.invitations[orgID]) == before {
		return ledger.ErrNotFound
	}
	return nil
}
