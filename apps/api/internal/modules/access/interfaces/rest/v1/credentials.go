package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/restapi"
)

// KeyPath names an API key by its ID.
type KeyPath struct {
	Key string `path:"key" doc:"The key ID, the middle of tbp_<id>_<secret>."`
}

// ClientPath names a machine client by its client ID.
type ClientPath struct {
	Client string `path:"client" doc:"The client ID."`
}

func registerCredentials(api huma.API, h *handlers) {
	keys := []string{"api-keys"}
	restapi.Register(api, huma.Operation{
		OperationID: "listAPIKeys", Method: http.MethodGet, Path: "/v1/api-keys", Tags: keys,
		Summary:     "List API keys",
		Description: "The organization's API keys, active ones first, without their secrets. Needs `org.credentials`.",
		Responses:   restapi.Success(http.StatusOK, "The keys."),
	}, h.listAPIKeys)
	restapi.Register(api, huma.Operation{
		OperationID: "createAPIKey", Method: http.MethodPost, Path: "/v1/api-keys", Tags: keys,
		Summary: "Create an API key",
		Description: "Creates a key with the ledger roles it acts with and answers its secret, shown once: the\n" +
			"platform keeps only a keyed hash of it. Send it as `Authorization: Bearer tbp_<id>_<secret>` on\n" +
			"every transport. Needs `org.credentials`.\n",
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The key and its secret."),
	}, h.createAPIKey)
	restapi.Register(api, huma.Operation{
		OperationID: "getAPIKey", Method: http.MethodGet, Path: "/v1/api-keys/{key}", Tags: keys,
		Summary: "Get an API key", Description: "One key, without its secret. Needs `org.credentials`.",
		Responses: restapi.Success(http.StatusOK, "The key."),
	}, h.getAPIKey)
	restapi.Register(api, huma.Operation{
		OperationID: "revokeAPIKey", Method: http.MethodDelete, Path: "/v1/api-keys/{key}", Tags: keys,
		Summary: "Revoke an API key",
		Description: "Stops the key from its next request; it cannot be undone. Its grants and the writes it made\n" +
			"stay, with the key as actor. Needs `org.credentials`.\n",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The key was revoked."),
	}, h.revokeAPIKey)
	restapi.Register(api, huma.Operation{
		OperationID: "setAPIKeyRole", Method: http.MethodPut, Path: "/v1/api-keys/{key}/ledgers/{ledger}", Tags: keys,
		Summary: "Set an API key's ledger role", Description: "Gives the key a role on the ledger. Needs `org.credentials`.",
		DefaultStatus: http.StatusNoContent, Responses: restapi.Success(http.StatusNoContent, "The role was set."),
	}, h.setAPIKeyRole)
	restapi.Register(api, huma.Operation{
		OperationID: "revokeAPIKeyRole", Method: http.MethodDelete, Path: "/v1/api-keys/{key}/ledgers/{ledger}", Tags: keys,
		Summary: "Revoke an API key's ledger role", Description: "Removes the key's role on the ledger. Needs `org.credentials`.",
		DefaultStatus: http.StatusNoContent, Responses: restapi.Success(http.StatusNoContent, "The role was revoked."),
	}, h.revokeAPIKeyRole)

	clients := []string{"machine-clients"}
	restapi.Register(api, huma.Operation{
		OperationID: "listMachineClients", Method: http.MethodGet, Path: "/v1/machine-clients", Tags: clients,
		Summary:     "List machine clients",
		Description: "The organization's OAuth clients in Keycloak, active ones first. Needs `org.credentials`.",
		Responses:   restapi.Success(http.StatusOK, "The clients."),
	}, h.listMachineClients)
	restapi.Register(api, huma.Operation{
		OperationID: "createMachineClient", Method: http.MethodPost, Path: "/v1/machine-clients", Tags: clients,
		Summary: "Create a machine client",
		Description: "Creates a confidential client in Keycloak, named <organization>-<name>, whose client credentials\n" +
			"tokens act for this organization with the ledger roles given. Answers the secret, shown once,\n" +
			"and the token URL. Needs `org.credentials`.\n",
		DefaultStatus: http.StatusCreated,
		Responses:     restapi.Success(http.StatusCreated, "The client and its secret."),
	}, h.createMachineClient)
	restapi.Register(api, huma.Operation{
		OperationID: "getMachineClient", Method: http.MethodGet, Path: "/v1/machine-clients/{client}", Tags: clients,
		Summary: "Get a machine client", Description: "One client, without its secret. Needs `org.credentials`.",
		Responses: restapi.Success(http.StatusOK, "The client."),
	}, h.getMachineClient)
	restapi.Register(api, huma.Operation{
		OperationID: "deleteMachineClient", Method: http.MethodDelete, Path: "/v1/machine-clients/{client}", Tags: clients,
		Summary: "Delete a machine client",
		Description: "Deletes the client in Keycloak, so it gets no new token; a token already issued works until it\n" +
			"expires, five minutes at most. Its grants and writes stay. Needs `org.credentials`.\n",
		DefaultStatus: http.StatusNoContent,
		Responses:     restapi.Success(http.StatusNoContent, "The client was deleted."),
	}, h.deleteMachineClient)
	restapi.Register(api, huma.Operation{
		OperationID: "regenerateMachineClientSecret", Method: http.MethodPost, Path: "/v1/machine-clients/{client}/secret", Tags: clients,
		Summary:     "Regenerate a machine client's secret",
		Description: "Replaces the secret; the old one stops working at once. Needs `org.credentials`.",
		Responses:   restapi.Success(http.StatusOK, "The new secret, shown once."),
	}, h.regenerateSecret)
	restapi.Register(api, huma.Operation{
		OperationID: "setMachineClientRole", Method: http.MethodPut, Path: "/v1/machine-clients/{client}/ledgers/{ledger}", Tags: clients,
		Summary: "Set a machine client's ledger role", Description: "Gives the client a role on the ledger. Needs `org.credentials`.",
		DefaultStatus: http.StatusNoContent, Responses: restapi.Success(http.StatusNoContent, "The role was set."),
	}, h.setMachineClientRole)
	restapi.Register(api, huma.Operation{
		OperationID: "revokeMachineClientRole", Method: http.MethodDelete, Path: "/v1/machine-clients/{client}/ledgers/{ledger}", Tags: clients,
		Summary: "Revoke a machine client's ledger role", Description: "Removes the client's role on the ledger. Needs `org.credentials`.",
		DefaultStatus: http.StatusNoContent, Responses: restapi.Success(http.StatusNoContent, "The role was revoked."),
	}, h.revokeMachineClientRole)
}

func (h *handlers) listAPIKeys(ctx context.Context, _ *struct{}) (*restapi.Body[dto.APIKeyList], error) {
	views, err := h.Credentials.APIKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.APIKeyList{Data: make([]dto.APIKey, len(views))}
	for i := range views {
		out.Data[i] = dto.APIKeyFrom(views[i])
	}
	return &restapi.Body[dto.APIKeyList]{Body: out}, nil
}

func (h *handlers) createAPIKey(ctx context.Context, in *restapi.Body[dto.CreateAPIKeyRequest]) (*restapi.Body[dto.CreatedAPIKey], error) {
	created, err := h.Credentials.CreateAPIKey(ctx, application.NewAPIKey{
		Name: in.Body.Name, ExpiresAt: in.Body.ExpiresAt, Ledgers: dto.LedgerGrantsOf(in.Body.Ledgers),
	})
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.CreatedAPIKey]{Body: dto.CreatedAPIKey{Key: dto.APIKeyFrom(created.Key), Secret: created.Secret}}, nil
}

func (h *handlers) getAPIKey(ctx context.Context, in *KeyPath) (*restapi.Body[dto.APIKey], error) {
	v, err := h.Credentials.APIKey(ctx, in.Key)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.APIKey]{Body: dto.APIKeyFrom(v)}, nil
}

func (h *handlers) revokeAPIKey(ctx context.Context, in *KeyPath) (*struct{}, error) {
	return &struct{}{}, h.Credentials.RevokeAPIKey(ctx, in.Key)
}

func (h *handlers) setAPIKeyRole(ctx context.Context, in *struct {
	KeyPath
	restapi.LedgerPath
	Body dto.LedgerRoleRequest
},
) (*struct{}, error) {
	return &struct{}{}, h.Credentials.SetCredentialRole(ctx, application.KindAPIKey, in.Key, string(in.Ledger), ledger.Role(in.Body.Role))
}

func (h *handlers) revokeAPIKeyRole(ctx context.Context, in *struct {
	KeyPath
	restapi.LedgerPath
},
) (*struct{}, error) {
	return &struct{}{}, h.Credentials.RevokeCredentialRole(ctx, application.KindAPIKey, in.Key, string(in.Ledger))
}

func (h *handlers) listMachineClients(ctx context.Context, _ *struct{}) (*restapi.Body[dto.MachineClientList], error) {
	views, err := h.Credentials.MachineClients(ctx)
	if err != nil {
		return nil, err
	}
	out := dto.MachineClientList{Data: make([]dto.MachineClient, len(views))}
	for i := range views {
		out.Data[i] = dto.MachineClientFrom(views[i])
	}
	return &restapi.Body[dto.MachineClientList]{Body: out}, nil
}

func (h *handlers) createMachineClient(ctx context.Context, in *restapi.Body[dto.CreateMachineClientRequest]) (*restapi.Body[dto.CreatedMachineClient], error) {
	created, err := h.Credentials.CreateMachineClient(ctx, application.NewMachineClient{Name: in.Body.Name, Ledgers: dto.LedgerGrantsOf(in.Body.Ledgers)})
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.CreatedMachineClient]{Body: dto.CreatedMachineClient{
		Client: dto.MachineClientFrom(created.Client), Secret: created.Secret, TokenURL: created.TokenURL,
	}}, nil
}

func (h *handlers) getMachineClient(ctx context.Context, in *ClientPath) (*restapi.Body[dto.MachineClient], error) {
	v, err := h.Credentials.MachineClient(ctx, in.Client)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.MachineClient]{Body: dto.MachineClientFrom(v)}, nil
}

func (h *handlers) deleteMachineClient(ctx context.Context, in *ClientPath) (*struct{}, error) {
	return &struct{}{}, h.Credentials.DeleteMachineClient(ctx, in.Client)
}

func (h *handlers) regenerateSecret(ctx context.Context, in *ClientPath) (*restapi.Body[dto.ClientSecret], error) {
	secret, err := h.Credentials.RegenerateSecret(ctx, in.Client)
	if err != nil {
		return nil, err
	}
	return &restapi.Body[dto.ClientSecret]{Body: dto.ClientSecret{Secret: secret}}, nil
}

func (h *handlers) setMachineClientRole(ctx context.Context, in *struct {
	ClientPath
	restapi.LedgerPath
	Body dto.LedgerRoleRequest
},
) (*struct{}, error) {
	return &struct{}{}, h.Credentials.SetCredentialRole(ctx, application.KindMachineClient, in.Client, string(in.Ledger), ledger.Role(in.Body.Role))
}

func (h *handlers) revokeMachineClientRole(ctx context.Context, in *struct {
	ClientPath
	restapi.LedgerPath
},
) (*struct{}, error) {
	return &struct{}{}, h.Credentials.RevokeCredentialRole(ctx, application.KindMachineClient, in.Client, string(in.Ledger))
}
