package application

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/domain"
)

// API keys and machine clients (ADR-0029): an organization's own credentials for machines. Both
// act with the ledger roles they are granted, like people; only org.credentials creates and
// removes them.

// CredentialStatus says whether a credential still works.
type CredentialStatus string

// The statuses of a credential.
const (
	CredentialActive  CredentialStatus = "active"
	CredentialExpired CredentialStatus = "expired"
	CredentialRevoked CredentialStatus = "revoked"
	CredentialDeleted CredentialStatus = "deleted"
)

// CredentialKind names the two kinds of credential.
type CredentialKind string

// The kinds of credential.
const (
	KindAPIKey        CredentialKind = "api-key"
	KindMachineClient CredentialKind = "machine-client"
)

// APIKeyView is an API key without its secret.
type APIKeyView struct {
	ID         string
	Name       string
	Status     CredentialStatus
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	CreatedBy  string
	Ledgers    []LedgerRoleView
}

// NewAPIKey is what an admin gives to create a key.
type NewAPIKey struct {
	Name      string
	ExpiresAt *time.Time
	Ledgers   []LedgerGrant
}

// CreatedAPIKey is a new key with its secret, shown once.
type CreatedAPIKey struct {
	Key APIKeyView
	// Secret is the whole key, tbp_<id>_<secret>.
	Secret string
}

// MachineClientView is a machine client without its secret.
type MachineClientView struct {
	ClientID    string
	Name        string
	Subject     string
	Status      CredentialStatus
	SecretSetAt time.Time
	LastTokenAt *time.Time
	DeletedAt   *time.Time
	CreatedAt   time.Time
	CreatedBy   string
	Ledgers     []LedgerRoleView
}

// NewMachineClient is what an admin gives to create a machine client.
type NewMachineClient struct {
	Name    string
	Ledgers []LedgerGrant
}

// CreatedMachineClient is a new client with its secret, shown once.
type CreatedMachineClient struct {
	Client   MachineClientView
	Secret   string
	TokenURL string
}

// credentials are the use cases of API keys and machine clients. Every one needs org.credentials.
type credentials struct {
	authz   authorizer
	grants  domain.GrantRepository
	keys    domain.APIKeyRepository
	clients domain.MachineClientRepository
	idp     domain.Clients
	dir     domain.Directory
	orgs    domain.OrganizationRepository
	// pepper keys the HMAC of API key secrets (AUTH_API_KEY_PEPPER); without it there are no keys.
	pepper []byte
	clock  ledger.Clock
}

// credentialNamePattern is the name format of migration 00014: names appear in client IDs, grants
// and logs.
var credentialNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

const apiKeyPrefix = "tbp_"

// APIKeys lists the organization's keys, active ones first.
func (c *credentials) APIKeys(ctx context.Context) ([]APIKeyView, error) {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return nil, err
	}
	keys, err := c.keys.APIKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("access: api keys: %w", err)
	}
	out := make([]APIKeyView, len(keys))
	for i := range keys {
		if out[i], err = c.apiKeyView(ctx, keys[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// APIKey returns one key.
func (c *credentials) APIKey(ctx context.Context, id string) (APIKeyView, error) {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return APIKeyView{}, err
	}
	k, err := c.keys.APIKey(ctx, id)
	if err != nil {
		return APIKeyView{}, fmt.Errorf("access: api key: %w", err)
	}
	return c.apiKeyView(ctx, k)
}

// CreateAPIKey creates a key with its ledger roles and returns its secret, which is not kept.
func (c *credentials) CreateAPIKey(ctx context.Context, in NewAPIKey) (CreatedAPIKey, error) {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return CreatedAPIKey{}, err
	}
	if len(c.pepper) == 0 {
		return CreatedAPIKey{}, ledger.NewProblem(ledger.ErrUnavailable, "dependency-unavailable", "API keys are not configured (AUTH_API_KEY_PEPPER)")
	}
	if err := validateCredential(in.Name, in.Ledgers); err != nil {
		return CreatedAPIKey{}, err
	}
	now := c.clock.Now()
	if in.ExpiresAt != nil && !in.ExpiresAt.After(now) {
		return CreatedAPIKey{}, invalid("the expiry must be in the future")
	}
	id, err := randomString(6)
	if err != nil {
		return CreatedAPIKey{}, err
	}
	secret, err := randomString(32)
	if err != nil {
		return CreatedAPIKey{}, err
	}
	// Base64 also uses - and _; the ID keeps letters and digits only, so the key splits cleanly.
	id = strings.NewReplacer("-", "a", "_", "b").Replace(id)[:8]
	p, _ := ledger.PrincipalFrom(ctx)
	k := domain.APIKey{ID: id, Name: in.Name, SecretHash: c.hash(id, secret), ExpiresAt: in.ExpiresAt, CreatedAt: now, CreatedBy: p.Name}
	if err := c.keys.CreateAPIKey(ctx, k, p); err != nil {
		if errors.Is(err, ledger.ErrAlreadyExists) {
			return CreatedAPIKey{}, ledger.NewProblem(ledger.ErrAlreadyExists, "already-exists", "an active key is already named "+in.Name)
		}
		return CreatedAPIKey{}, fmt.Errorf("access: create api key: %w", err)
	}
	holder := domain.Holder{Issuer: domain.APIKeyIssuer, Subject: id}
	if err := c.grantAll(ctx, holder, in.Ledgers, p); err != nil {
		// A key without its roles would act with none: take it back.
		_ = c.keys.RevokeAPIKey(ctx, id, now)
		return CreatedAPIKey{}, err
	}
	view, err := c.apiKeyView(ctx, k)
	if err != nil {
		return CreatedAPIKey{}, err
	}
	return CreatedAPIKey{Key: view, Secret: apiKeyPrefix + id + "_" + secret}, nil
}

// RevokeAPIKey stops a key at once. The key, its grants and its writes stay, as history.
func (c *credentials) RevokeAPIKey(ctx context.Context, id string) error {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return err
	}
	if err := c.keys.RevokeAPIKey(ctx, id, c.clock.Now()); err != nil {
		return fmt.Errorf("access: revoke api key: %w", err)
	}
	return nil
}

// MachineClients lists the organization's machine clients, active ones first.
func (c *credentials) MachineClients(ctx context.Context) ([]MachineClientView, error) {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return nil, err
	}
	clients, err := c.clients.MachineClients(ctx)
	if err != nil {
		return nil, fmt.Errorf("access: machine clients: %w", err)
	}
	out := make([]MachineClientView, len(clients))
	for i := range clients {
		if out[i], err = c.machineClientView(ctx, clients[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// MachineClient returns one machine client.
func (c *credentials) MachineClient(ctx context.Context, clientID string) (MachineClientView, error) {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return MachineClientView{}, err
	}
	m, err := c.clients.MachineClient(ctx, clientID)
	if err != nil {
		return MachineClientView{}, fmt.Errorf("access: machine client: %w", err)
	}
	return c.machineClientView(ctx, m)
}

// CreateMachineClient creates a Keycloak client named <organization>-<name>, records it for the
// organization, grants its roles and returns its secret.
func (c *credentials) CreateMachineClient(ctx context.Context, in NewMachineClient) (CreatedMachineClient, error) {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return CreatedMachineClient{}, err
	}
	if c.idp == nil {
		return CreatedMachineClient{}, unconfigured()
	}
	if err := validateCredential(in.Name, in.Ledgers); err != nil {
		return CreatedMachineClient{}, err
	}
	t, err := ledger.RequireTenant(ctx)
	if err != nil {
		return CreatedMachineClient{}, err
	}
	clientID := t.Alias + "-" + in.Name
	created, err := c.idp.CreateClient(ctx, domain.NewClient{ClientID: clientID, Name: in.Name})
	if errors.Is(err, ledger.ErrAlreadyExists) {
		return CreatedMachineClient{}, ledger.NewProblem(ledger.ErrAlreadyExists, "already-exists", "a client "+clientID+" already exists")
	}
	if err != nil {
		return CreatedMachineClient{}, fmt.Errorf("access: create client: %w", err)
	}
	p, _ := ledger.PrincipalFrom(ctx)
	now := c.clock.Now()
	m := domain.MachineClient{
		ClientID: clientID, KeycloakID: created.KeycloakID, Subject: created.Subject, Name: in.Name,
		SecretSetAt: now, CreatedAt: now, CreatedBy: p.Name,
	}
	if err := c.clients.CreateMachineClient(ctx, m, p); err != nil {
		_ = c.idp.DeleteClient(ctx, created.KeycloakID)
		return CreatedMachineClient{}, fmt.Errorf("access: record machine client: %w", err)
	}
	if err := c.grantAll(ctx, domain.Holder{Issuer: c.dir.Issuer(), Subject: created.Subject}, in.Ledgers, p); err != nil {
		_ = c.idp.DeleteClient(ctx, created.KeycloakID)
		_ = c.clients.DeleteMachineClient(ctx, clientID, now)
		return CreatedMachineClient{}, err
	}
	view, err := c.machineClientView(ctx, m)
	if err != nil {
		return CreatedMachineClient{}, err
	}
	return CreatedMachineClient{Client: view, Secret: created.Secret, TokenURL: c.idp.TokenURL()}, nil
}

// RegenerateSecret replaces a machine client's secret; the old one stops at once.
func (c *credentials) RegenerateSecret(ctx context.Context, clientID string) (string, error) {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return "", err
	}
	if c.idp == nil {
		return "", unconfigured()
	}
	m, err := c.clients.MachineClient(ctx, clientID)
	if err != nil {
		return "", fmt.Errorf("access: machine client: %w", err)
	}
	if m.DeletedAt != nil {
		return "", fmt.Errorf("access: machine client %q: %w", clientID, ledger.ErrNotFound)
	}
	secret, err := c.idp.RegenerateSecret(ctx, m.KeycloakID)
	if err != nil {
		return "", fmt.Errorf("access: regenerate secret: %w", err)
	}
	if err := c.clients.SetSecretSetAt(ctx, clientID, c.clock.Now()); err != nil {
		return "", fmt.Errorf("access: record secret: %w", err)
	}
	return secret, nil
}

// DeleteMachineClient deletes the Keycloak client, so it gets no new token, and marks the
// platform's record deleted; its grants and writes stay as history.
func (c *credentials) DeleteMachineClient(ctx context.Context, clientID string) error {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return err
	}
	if c.idp == nil {
		return unconfigured()
	}
	m, err := c.clients.MachineClient(ctx, clientID)
	if err != nil {
		return fmt.Errorf("access: machine client: %w", err)
	}
	if err := c.idp.DeleteClient(ctx, m.KeycloakID); err != nil && !errors.Is(err, ledger.ErrNotFound) {
		return fmt.Errorf("access: delete client: %w", err)
	}
	if err := c.clients.DeleteMachineClient(ctx, clientID, c.clock.Now()); err != nil {
		return fmt.Errorf("access: delete machine client: %w", err)
	}
	return nil
}

// SetCredentialRole gives a key or a client a role on one ledger.
func (c *credentials) SetCredentialRole(ctx context.Context, kind CredentialKind, id, ledgerName string, role ledger.Role) error {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return err
	}
	if !isOneOf(role, ledger.LedgerRoles) {
		return invalid("role must be viewer, operator or admin")
	}
	holder, err := c.holder(ctx, kind, id)
	if err != nil {
		return err
	}
	p, _ := ledger.PrincipalFrom(ctx)
	return c.grantAll(ctx, holder, []LedgerGrant{{Ledger: ledgerName, Role: role}}, p)
}

// RevokeCredentialRole removes a key's or a client's role on one ledger.
func (c *credentials) RevokeCredentialRole(ctx context.Context, kind CredentialKind, id, ledgerName string) error {
	if err := c.authz.Require(ctx, ledger.PermOrgCredentials, ""); err != nil {
		return err
	}
	holder, err := c.holder(ctx, kind, id)
	if err != nil {
		return err
	}
	if err := c.grants.RevokeGrant(ctx, holder, ledgerName); err != nil {
		return fmt.Errorf("access: revoke: %w", err)
	}
	return nil
}

// holder is the principal a credential acts as, for its grants.
func (c *credentials) holder(ctx context.Context, kind CredentialKind, id string) (domain.Holder, error) {
	switch kind {
	case KindAPIKey:
		k, err := c.keys.APIKey(ctx, id)
		if err != nil {
			return domain.Holder{}, fmt.Errorf("access: api key: %w", err)
		}
		return domain.Holder{Issuer: domain.APIKeyIssuer, Subject: k.ID}, nil
	case KindMachineClient:
		m, err := c.clients.MachineClient(ctx, id)
		if err != nil {
			return domain.Holder{}, fmt.Errorf("access: machine client: %w", err)
		}
		return domain.Holder{Issuer: c.dir.Issuer(), Subject: m.Subject}, nil
	default:
		return domain.Holder{}, invalid("kind must be api-key or machine-client")
	}
}

func (c *credentials) grantAll(ctx context.Context, h domain.Holder, ledgers []LedgerGrant, by ledger.Principal) error {
	for _, lg := range ledgers {
		g := domain.HeldGrant{Holder: h, Grant: domain.Grant{Ledger: lg.Ledger, Role: lg.Role}}
		if err := c.grants.PutGrant(ctx, g, by); err != nil {
			return fmt.Errorf("access: grant %s on %s: %w", lg.Role, lg.Ledger, err)
		}
	}
	return nil
}

func (c *credentials) ledgerRoles(ctx context.Context, h domain.Holder) ([]LedgerRoleView, error) {
	held, err := c.grants.GrantsIn(ctx)
	if err != nil {
		return nil, fmt.Errorf("access: grants: %w", err)
	}
	out := []LedgerRoleView{}
	for _, g := range held {
		if g.Holder == h && g.Ledger != "" {
			out = append(out, LedgerRoleView{Ledger: g.Ledger, Role: g.Role, GrantedAt: g.GrantedAt, GrantedBy: g.GrantedByName})
		}
	}
	return out, nil
}

func (c *credentials) apiKeyView(ctx context.Context, k domain.APIKey) (APIKeyView, error) {
	roles, err := c.ledgerRoles(ctx, domain.Holder{Issuer: domain.APIKeyIssuer, Subject: k.ID})
	if err != nil {
		return APIKeyView{}, err
	}
	status := CredentialActive
	switch {
	case k.RevokedAt != nil:
		status = CredentialRevoked
	case k.ExpiresAt != nil && !c.clock.Now().Before(*k.ExpiresAt):
		status = CredentialExpired
	}
	return APIKeyView{
		ID: k.ID, Name: k.Name, Status: status, ExpiresAt: k.ExpiresAt, LastUsedAt: k.LastUsedAt, RevokedAt: k.RevokedAt,
		CreatedAt: k.CreatedAt, CreatedBy: k.CreatedBy, Ledgers: roles,
	}, nil
}

func (c *credentials) machineClientView(ctx context.Context, m domain.MachineClient) (MachineClientView, error) {
	roles, err := c.ledgerRoles(ctx, domain.Holder{Issuer: c.dir.Issuer(), Subject: m.Subject})
	if err != nil {
		return MachineClientView{}, err
	}
	status := CredentialActive
	if m.DeletedAt != nil {
		status = CredentialDeleted
	}
	return MachineClientView{
		ClientID: m.ClientID, Name: m.Name, Subject: m.Subject, Status: status, SecretSetAt: m.SecretSetAt,
		LastTokenAt: m.LastTokenAt, DeletedAt: m.DeletedAt, CreatedAt: m.CreatedAt, CreatedBy: m.CreatedBy, Ledgers: roles,
	}, nil
}

// hash is the HMAC-SHA-256 of the key's ID and secret under the pepper (ADR-0029): 256 random
// bits need no slow hash, and without the pepper a copy of the table verifies nothing.
func (c *credentials) hash(id, secret string) []byte {
	mac := hmac.New(sha256.New, c.pepper)
	mac.Write([]byte(id + "." + secret))
	return mac.Sum(nil)
}

func validateCredential(name string, ledgers []LedgerGrant) error {
	if !credentialNamePattern.MatchString(name) {
		return invalid("name must be 1 to 63 lowercase letters, digits and dashes, starting with a letter or digit")
	}
	if len(ledgers) == 0 {
		return invalid("give it at least one ledger role")
	}
	for _, lg := range ledgers {
		if !isOneOf(lg.Role, ledger.LedgerRoles) {
			return invalid("ledger roles must be viewer, operator or admin")
		}
	}
	return nil
}

// randomString returns n random bytes in unpadded base64url.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("access: random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// credentialResolver turns a credential into a principal and its organization, for
// authentication: it checks no permission, since nobody is known yet.
type credentialResolver struct {
	keys    domain.APIKeyRepository
	clients domain.MachineClientRepository
	cred    *credentials
	clock   ledger.Clock
}

var errBadKey = ledger.NewProblem(ledger.ErrUnauthenticated, "unauthenticated", "the API key is not valid: it may have been revoked or expired")

// IsAPIKey reports whether a bearer value is an API key rather than a token.
func IsAPIKey(raw string) bool {
	return strings.HasPrefix(raw, apiKeyPrefix)
}

// APIKey checks a key and returns its principal and organization. A suspended organization is 403.
func (r credentialResolver) APIKey(ctx context.Context, raw string) (ledger.Principal, ledger.Tenant, error) {
	parts := strings.SplitN(raw, "_", 3)
	if len(r.cred.pepper) == 0 || len(parts) != 3 || parts[0]+"_" != apiKeyPrefix || len(parts[1]) != 8 {
		return ledger.Principal{}, ledger.Tenant{}, errBadKey
	}
	id, secret := parts[1], parts[2]
	k, err := r.keys.APIKeyForAuthentication(ctx, id)
	if errors.Is(err, ledger.ErrNotFound) {
		return ledger.Principal{}, ledger.Tenant{}, errBadKey
	}
	if err != nil {
		return ledger.Principal{}, ledger.Tenant{}, fmt.Errorf("access: api key: %w", err)
	}
	now := r.clock.Now()
	if !hmac.Equal(k.SecretHash, r.cred.hash(id, secret)) || k.RevokedAt != nil || (k.ExpiresAt != nil && !now.Before(*k.ExpiresAt)) {
		return ledger.Principal{}, ledger.Tenant{}, errBadKey
	}
	if err := suspended(k.Organization); err != nil {
		return ledger.Principal{}, ledger.Tenant{}, err
	}
	// Recording the use is best effort: a key works even when the write fails.
	_ = r.keys.TouchAPIKey(ctx, id, now)
	p := ledger.Principal{Issuer: domain.APIKeyIssuer, Subject: id, Kind: ledger.KindAPIKey, Name: k.Name}
	return p, k.Organization.Tenant(), nil
}

// MachineTenant returns the organization a machine token acts for: the one its client was created
// in. A client the platform did not create, or deleted, is refused.
func (r credentialResolver) MachineTenant(ctx context.Context, p ledger.Principal) (ledger.Tenant, error) {
	m, err := r.clients.MachineClientForAuthentication(ctx, p.Client)
	if errors.Is(err, ledger.ErrNotFound) || err == nil && (m.DeletedAt != nil || m.Subject != p.Subject) {
		return ledger.Tenant{}, ledger.NewProblem(ledger.ErrUnauthenticated, "unauthenticated",
			"the client "+p.Client+" is not a machine client of an organization")
	}
	if err != nil {
		return ledger.Tenant{}, fmt.Errorf("access: machine client: %w", err)
	}
	if err := suspended(m.Organization); err != nil {
		return ledger.Tenant{}, err
	}
	_ = r.clients.TouchMachineClient(ctx, p.Client, r.clock.Now())
	return m.Organization.Tenant(), nil
}

func suspended(o domain.Organization) error {
	if o.Status != domain.OrganizationSuspended {
		return nil
	}
	detail := "the organization " + o.Alias + " is suspended"
	if o.SuspensionReason != "" {
		detail += ": " + o.SuspensionReason
	}
	return ledger.NewProblem(ledger.ErrForbidden, "organization-suspended", detail)
}
