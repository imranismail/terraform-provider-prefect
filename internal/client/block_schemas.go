package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/prefecthq/terraform-provider-prefect/internal/api"
)

// BlockSchemaClient is a client for working with block schemas.
type BlockSchemaClient struct {
	hc           *http.Client
	routePrefix  string
	apiKey       string
	basicAuthKey string
}

// BlockSchemas returns a BlockSchemaClient.
//
//nolint:ireturn // required to support PrefectClient mocking
func (c *Client) BlockSchemas(accountID uuid.UUID, workspaceID uuid.UUID) (api.BlockSchemaClient, error) {
	if accountID == uuid.Nil {
		accountID = c.defaultAccountID
	}
	if workspaceID == uuid.Nil {
		workspaceID = c.defaultWorkspaceID
	}

	if err := validateCloudEndpoint(c.endpoint, accountID, workspaceID); err != nil {
		return nil, err
	}

	return &BlockSchemaClient{
		hc:           c.hc,
		apiKey:       c.apiKey,
		basicAuthKey: c.basicAuthKey,
		routePrefix:  getWorkspaceScopedURL(c.endpoint, accountID, workspaceID, "block_schemas"),
	}, nil
}

// List gets a list of BlockSchemas for a given list of block type slugs.
func (c *BlockSchemaClient) List(ctx context.Context, blockTypeIDs []uuid.UUID) ([]*api.BlockSchema, error) {
	filterQuery := &api.BlockSchemaFilter{}
	filterQuery.BlockSchemas.BlockTypeID.Any = blockTypeIDs

	cfg := requestConfig{
		method:       http.MethodPost,
		url:          c.routePrefix + "/filter",
		body:         filterQuery,
		apiKey:       c.apiKey,
		basicAuthKey: c.basicAuthKey,
		successCodes: successCodesStatusOK,
	}

	var blockSchemas []*api.BlockSchema
	if err := requestWithDecodeResponse(ctx, c.hc, cfg, &blockSchemas); err != nil {
		return nil, fmt.Errorf("failed to get block schemas: %w", err)
	}

	return blockSchemas, nil
}
