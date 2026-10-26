// Package keycloak is a client of Keycloak's admin REST API: organizations, users, memberships and
// invitations (ADR-0027). It is the only code that knows that API; the access module adapts it to
// its Directory port, and the rest of the platform speaks standard OpenID Connect (ADR-0026).
//
// The API calls it as a service account (the client credentials grant) holding only
// realm-management's manage-organizations, view-organizations, manage-users, view-users and
// query-users roles. Keycloak's own error text never leaves this package: errors carry the status
// and one of the kernel's sentinels.
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
)

// Config locates the realm and the service account.
type Config struct {
	// RealmURL is the realm as the API reaches it, such as http://keycloak:8080/realms/tbp: the
	// discovery URL without its well-known suffix. The admin API is the same path under /admin.
	RealmURL     string
	ClientID     string
	ClientSecret string
	// SetupClientID is the client whose pages follow the account setup email: the console's.
	SetupClientID string
	// HTTPClient carries the requests; nil means a client with a ten second timeout.
	HTTPClient *http.Client
}

// SetupActions are what a person the platform created must do before signing in: verify the
// email and set a password.
var SetupActions = []string{"VERIFY_EMAIL", "UPDATE_PASSWORD"}

// setupLifespan is how long the account setup link stays valid.
const setupLifespan = 7 * 24 * time.Hour

// pageSize is how many members or invitations one admin call returns.
const pageSize = 100

// maxBody bounds what is read of one answer.
const maxBody = 4 << 20

// Admin is a client of one realm's admin API.
type Admin struct {
	admin string
	setup string
	http  *http.Client
}

// New builds the directory. It makes no call: the first request fetches the service account's
// token, and the oauth2 package renews it before it expires.
func New(cfg Config) (*Admin, error) {
	realm, err := url.Parse(strings.TrimSuffix(cfg.RealmURL, "/"))
	if err != nil || realm.Scheme == "" || realm.Host == "" {
		return nil, fmt.Errorf("keycloak: realm URL %q is not an absolute URL", cfg.RealmURL)
	}
	dir, name := path.Split(realm.Path)
	if path.Base(strings.TrimSuffix(dir, "/")) != "realms" || name == "" {
		return nil, fmt.Errorf("keycloak: realm URL %q does not end with /realms/<realm>", cfg.RealmURL)
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("keycloak: the admin client ID and secret are required")
	}
	base := cfg.HTTPClient
	if base == nil {
		base = &http.Client{Timeout: 10 * time.Second}
	}
	tokens := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     realm.String() + "/protocol/openid-connect/token",
	}
	// Go note: the oauth2 package takes its HTTP client from the context it is given; the
	// returned client adds the service account's bearer token to every request.
	authed := tokens.Client(context.WithValue(context.Background(), oauth2.HTTPClient, base))
	authed.Timeout = base.Timeout
	admin := *realm
	admin.Path = path.Join(strings.TrimSuffix(dir, "realms/"), "admin", "realms", name)
	return &Admin{admin: admin.String(), setup: cfg.SetupClientID, http: authed}, nil
}

// Organization is a Keycloak organization.
type Organization struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name"`
	Alias   string `json:"alias"`
	Enabled bool   `json:"enabled"`
}

// OrganizationID returns the ID of the organization with the alias. Keycloak's q=alias:<alias>
// search matches the alias exactly; the alias is compared again here all the same.
func (d *Admin) OrganizationID(ctx context.Context, alias string) (string, error) {
	var orgs []Organization
	q := url.Values{"q": {"alias:" + alias}, "briefRepresentation": {"true"}}
	if err := d.do(ctx, http.MethodGet, "/organizations?"+q.Encode(), nil, &orgs); err != nil {
		return "", fmt.Errorf("keycloak: organization %q: %w", alias, err)
	}
	for _, o := range orgs {
		if o.Alias == alias {
			return o.ID, nil
		}
	}
	return "", fmt.Errorf("keycloak: organization %q: %w", alias, ledger.ErrNotFound)
}

// CreateOrganization creates an enabled organization without domains.
func (d *Admin) CreateOrganization(ctx context.Context, alias, name string) (string, error) {
	loc, err := d.create(ctx, "/organizations", Organization{Alias: alias, Name: name, Enabled: true})
	if err != nil {
		return "", fmt.Errorf("keycloak: create organization %q: %w", alias, err)
	}
	return loc, nil
}

// RenameOrganization changes the name. The representation is read and written back as a map, so
// what this package does not model (domains, attributes, the redirect URL) stays as it is.
func (d *Admin) RenameOrganization(ctx context.Context, orgID, alias, name string) error {
	var rep map[string]json.RawMessage
	if err := d.do(ctx, http.MethodGet, "/organizations/"+url.PathEscape(orgID), nil, &rep); err != nil {
		return fmt.Errorf("keycloak: organization %q: %w", alias, err)
	}
	encoded, err := json.Marshal(name)
	if err != nil {
		return fmt.Errorf("keycloak: rename %q: %w", alias, err)
	}
	rep["name"] = encoded
	if err := d.do(ctx, http.MethodPut, "/organizations/"+url.PathEscape(orgID), rep, nil); err != nil {
		return fmt.Errorf("keycloak: rename %q: %w", alias, err)
	}
	return nil
}

// User is a Keycloak user. SetupPending reports whether required actions remain.
type User struct {
	ID              string   `json:"id,omitempty"`
	Username        string   `json:"username,omitempty"`
	Email           string   `json:"email"`
	FirstName       string   `json:"firstName,omitempty"`
	LastName        string   `json:"lastName,omitempty"`
	Enabled         bool     `json:"enabled"`
	EmailVerified   bool     `json:"emailVerified"`
	RequiredActions []string `json:"requiredActions,omitempty"`
}

// SetupPending reports whether the user has required actions left, such as those of
// CreateUser.
func (u User) SetupPending() bool { return len(u.RequiredActions) > 0 }

// UserByEmail finds the user with the email. Keycloak stores emails in lower case.
func (d *Admin) UserByEmail(ctx context.Context, email string) (User, error) {
	var users []User
	q := url.Values{"email": {email}, "exact": {"true"}, "briefRepresentation": {"false"}}
	if err := d.do(ctx, http.MethodGet, "/users?"+q.Encode(), nil, &users); err != nil {
		return User{}, fmt.Errorf("keycloak: user by email: %w", err)
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return User{}, fmt.Errorf("keycloak: user by email: %w", ledger.ErrNotFound)
}

// CreateUser creates an account with the email as username and SetupActions required, so it
// cannot sign in until its owner follows the setup email.
func (d *Admin) CreateUser(ctx context.Context, email, firstName, lastName string) (User, error) {
	u := User{
		Username: email, Email: email, FirstName: firstName, LastName: lastName,
		Enabled: true, RequiredActions: SetupActions,
	}
	id, err := d.create(ctx, "/users", u)
	if err != nil {
		return User{}, fmt.Errorf("keycloak: create user: %w", err)
	}
	u.ID = id
	return u, nil
}

// SendAccountSetup emails the link to SetupActions, valid for setupLifespan, through the setup
// client's pages.
func (d *Admin) SendAccountSetup(ctx context.Context, userID string) error {
	q := url.Values{"lifespan": {strconv.Itoa(int(setupLifespan.Seconds()))}}
	if d.setup != "" {
		q.Set("client_id", d.setup)
	}
	p := "/users/" + url.PathEscape(userID) + "/execute-actions-email?" + q.Encode()
	if err := d.do(ctx, http.MethodPut, p, SetupActions, nil); err != nil {
		return fmt.Errorf("keycloak: account setup email: %w", err)
	}
	return nil
}

// Members lists the members of the organization, a page at a time.
func (d *Admin) Members(ctx context.Context, orgID string) ([]User, error) {
	var out []User
	for first := 0; ; first += pageSize {
		var users []User
		q := url.Values{"first": {strconv.Itoa(first)}, "max": {strconv.Itoa(pageSize)}, "briefRepresentation": {"false"}}
		if err := d.do(ctx, http.MethodGet, "/organizations/"+url.PathEscape(orgID)+"/members?"+q.Encode(), nil, &users); err != nil {
			return nil, fmt.Errorf("keycloak: members: %w", err)
		}
		out = append(out, users...)
		if len(users) < pageSize {
			return out, nil
		}
	}
}

// Member returns the user when they are a member of the organization, ErrNotFound otherwise.
func (d *Admin) Member(ctx context.Context, orgID, userID string) (User, error) {
	var u User
	if err := d.do(ctx, http.MethodGet, "/organizations/"+url.PathEscape(orgID)+"/members/"+url.PathEscape(userID), nil, &u); err != nil {
		return User{}, fmt.Errorf("keycloak: member: %w", err)
	}
	return u, nil
}

// AddMember makes the user an unmanaged member, which removing never deletes. Keycloak's body is
// the bare user ID as a JSON string.
func (d *Admin) AddMember(ctx context.Context, orgID, userID string) error {
	if err := d.do(ctx, http.MethodPost, "/organizations/"+url.PathEscape(orgID)+"/members", userID, nil); err != nil {
		return fmt.Errorf("keycloak: add member: %w", err)
	}
	return nil
}

// RemoveMember ends an unmanaged membership; the account stays.
func (d *Admin) RemoveMember(ctx context.Context, orgID, userID string) error {
	if err := d.do(ctx, http.MethodDelete, "/organizations/"+url.PathEscape(orgID)+"/members/"+url.PathEscape(userID), nil, nil); err != nil {
		return fmt.Errorf("keycloak: remove member: %w", err)
	}
	return nil
}

// Invite emails an existing user Keycloak's invitation to the organization; following it makes
// them an unmanaged member.
func (d *Admin) Invite(ctx context.Context, orgID, userID string) error {
	form := url.Values{"id": {userID}}
	if err := d.do(ctx, http.MethodPost, "/organizations/"+url.PathEscape(orgID)+"/members/invite-existing-user", form, nil); err != nil {
		return fmt.Errorf("keycloak: invite: %w", err)
	}
	return nil
}

// Invitation is an invitation not yet accepted.
type Invitation struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// SentDate and ExpiresAt are Unix seconds.
	SentDate  int64 `json:"sentDate"`
	ExpiresAt int64 `json:"expiresAt"`
}

// Invitations lists the invitations not yet accepted, expired ones included.
func (d *Admin) Invitations(ctx context.Context, orgID string) ([]Invitation, error) {
	var out []Invitation
	for first := 0; ; first += pageSize {
		var invs []Invitation
		q := url.Values{"first": {strconv.Itoa(first)}, "max": {strconv.Itoa(pageSize)}}
		if err := d.do(ctx, http.MethodGet, "/organizations/"+url.PathEscape(orgID)+"/invitations?"+q.Encode(), nil, &invs); err != nil {
			return nil, fmt.Errorf("keycloak: invitations: %w", err)
		}
		out = append(out, invs...)
		if len(invs) < pageSize {
			return out, nil
		}
	}
}

// ResendInvitation sends the invitation again.
func (d *Admin) ResendInvitation(ctx context.Context, orgID, invitationID string) error {
	p := "/organizations/" + url.PathEscape(orgID) + "/invitations/" + url.PathEscape(invitationID) + "/resend"
	if err := d.do(ctx, http.MethodPost, p, nil, nil); err != nil {
		return fmt.Errorf("keycloak: resend invitation: %w", err)
	}
	return nil
}

// DeleteInvitation withdraws the invitation.
func (d *Admin) DeleteInvitation(ctx context.Context, orgID, invitationID string) error {
	p := "/organizations/" + url.PathEscape(orgID) + "/invitations/" + url.PathEscape(invitationID)
	if err := d.do(ctx, http.MethodDelete, p, nil, nil); err != nil {
		return fmt.Errorf("keycloak: delete invitation: %w", err)
	}
	return nil
}

// create posts a representation and returns the ID at the end of the Location header.
func (d *Admin) create(ctx context.Context, p string, body any) (string, error) {
	res, err := d.send(ctx, http.MethodPost, p, body)
	if err != nil {
		return "", err
	}
	defer drain(res)
	if err := statusError(res); err != nil {
		return "", err
	}
	loc := res.Header.Get("Location")
	if loc == "" {
		return "", errors.New("created without a Location header")
	}
	return path.Base(loc), nil
}

// do sends a request and decodes a JSON answer into out, when out is not nil.
func (d *Admin) do(ctx context.Context, method, p string, body, out any) error {
	res, err := d.send(ctx, method, p, body)
	if err != nil {
		return err
	}
	defer drain(res)
	if err := statusError(res); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(out); err != nil {
		return fmt.Errorf("decode the answer: %w", err)
	}
	return nil
}

// send encodes the body: url.Values as a form, anything else as JSON.
func (d *Admin) send(ctx context.Context, method, p string, body any) (*http.Response, error) {
	var (
		reader      io.Reader
		contentType string
	)
	switch b := body.(type) {
	case nil:
	case url.Values:
		reader, contentType = strings.NewReader(b.Encode()), "application/x-www-form-urlencoded"
	default:
		encoded, err := json.Marshal(b)
		if err != nil {
			return nil, fmt.Errorf("encode the request: %w", err)
		}
		reader, contentType = bytes.NewReader(encoded), "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, method, d.admin+p, reader)
	if err != nil {
		return nil, fmt.Errorf("build the request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	res, err := d.http.Do(req)
	if err != nil {
		// The provider did not answer, or its token endpoint refused the service account.
		return nil, fmt.Errorf("%w: %w", ledger.ErrUnavailable, err)
	}
	return res, nil
}

// statusError maps Keycloak's status to the kernel's sentinels, without its message.
func statusError(res *http.Response) error {
	switch {
	case res.StatusCode < 300:
		return nil
	case res.StatusCode == http.StatusNotFound:
		return fmt.Errorf("status %d: %w", res.StatusCode, ledger.ErrNotFound)
	case res.StatusCode == http.StatusConflict:
		return fmt.Errorf("status %d: %w", res.StatusCode, ledger.ErrAlreadyExists)
	case res.StatusCode >= 500:
		return fmt.Errorf("status %d: %w", res.StatusCode, ledger.ErrUnavailable)
	default:
		// 400, 401 and 403 mean the request or the service account's roles are wrong: a bug or a
		// misconfiguration, not something the caller can fix.
		return fmt.Errorf("keycloak answered status %d", res.StatusCode)
	}
}

// drain reads what is left of the body so the connection can be reused.
func drain(res *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxBody))
	_ = res.Body.Close()
}
