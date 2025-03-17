package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/prefecthq/terraform-provider-prefect/internal/provider/helpers"
)

// getAccountScopedURL constructs a URL for an account-scoped route.
func getAccountScopedURL(endpoint string, accountID uuid.UUID, route string) string {
	var builder strings.Builder

	builder.WriteString(endpoint)

	builder.WriteString("/accounts/")
	builder.WriteString(accountID.String())
	builder.WriteRune('/')

	builder.WriteString(route)

	return builder.String()
}

// getWorkspaceScopedURL constructs a URL for a workspace-scoped route.
func getWorkspaceScopedURL(endpoint string, accountID uuid.UUID, workspaceID uuid.UUID, route string) string {
	var builder strings.Builder

	builder.WriteString(endpoint)

	if accountID != uuid.Nil && workspaceID != uuid.Nil {
		builder.WriteString("/accounts/")
		builder.WriteString(accountID.String())

		builder.WriteString("/workspaces/")
		builder.WriteString(workspaceID.String())
	}

	builder.WriteRune('/')
	builder.WriteString(route)

	return builder.String()
}

// setAuthorizationHeader will set the Authorization header to the
// provided apiKey, if set.
func setAuthorizationHeader(request *http.Request, apiKey, basicAuthKey string) {
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	if basicAuthKey != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(basicAuthKey))
		request.Header.Set("Authorization", "Basic "+encoded)
	}
}

// setDefaultHeaders will set Authorization, Content-Type, and Accept
// headers that are common to most requests.
func setDefaultHeaders(request *http.Request, apiKey, basicAuthKey string) {
	setAuthorizationHeader(request, apiKey, basicAuthKey)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
}

// validateCloudEndpoint validates that proper configuration is provided
// when the endpoint points to Prefect Cloud.
func validateCloudEndpoint(endpoint string, accountID, workspaceID uuid.UUID) error {
	if helpers.IsCloudEndpoint(endpoint) && (accountID == uuid.Nil || workspaceID == uuid.Nil) {
		return fmt.Errorf("prefect Cloud endpoints require an account_id and workspace_id to be set on either the provider or the resource")
	}

	return nil
}

// requestConfig is a configuration object for an HTTP request.
type requestConfig struct {
	method string
	url    string
	body   any

	successCodes []int

	apiKey       string
	basicAuthKey string
}

var (
	// successCodesStatusOK is a convenience variable to use for the most common
	// success criteria.
	successCodesStatusOK = []int{http.StatusOK}

	// successCodesStatusCreated is a convenience variable to use for a common
	// success criteria of StatusCreated.
	successCodesStatusCreated = []int{http.StatusCreated}

	// successCodesStatusNoContent is a convenience variable to use for a common
	// success criteria of StatusNoContent.
	successCodesStatusNoContent = []int{http.StatusNoContent}

	// successCodesStatusOKOrNoContent is a convenience variable to use for a common
	// success criteria of either StatusOK or StatusNoContent.
	successCodesStatusOKOrNoContent = []int{http.StatusOK, http.StatusNoContent}

	// successCodesStatusOKOrCreated is a convenience variable to use for a common
	// success criteria of either Status OK or StatusCreated.
	successCodesStatusOKOrCreated = []int{http.StatusOK, http.StatusCreated}
)

// request performs an HTTP request with the provided configuration.
// It returns the response, or an error if the request fails.
// The caller is responsible for closing the response body.
func request(ctx context.Context, client *http.Client, cfg requestConfig) (*http.Response, error) {
	var body io.Reader

	if cfg.body != nil && cfg.body != http.NoBody {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(cfg.body); err != nil {
			return nil, fmt.Errorf("failed to encode body data: %w", err)
		}

		body = &buf
	} else {
		body = http.NoBody
	}

	req, err := http.NewRequestWithContext(ctx, cfg.method, cfg.url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	setDefaultHeaders(req, cfg.apiKey, cfg.basicAuthKey)

	// Body will be closed by the caller.
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}

	success := false
	for _, successCode := range cfg.successCodes {
		if resp.StatusCode == successCode {
			success = true

			break
		}
	}

	if !success {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("status code=%s, error=%s", resp.Status, body)
	}

	return resp, nil
}

// decodeResponseBody decodes the response body into the target object.
// It returns an error if the decoding fails.
func decodeResponseBody(respBody io.ReadCloser, target any) error {
	if err := json.NewDecoder(respBody).Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// requestWithDecodeResponse performs an HTTP request with the provided configuration,
// and decodes the response body into the target object.
// It returns an error if the request fails or the decoding fails.
func requestWithDecodeResponse(ctx context.Context, client *http.Client, cfg requestConfig, target any) error {
	resp, err := request(ctx, client, cfg)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if err := decodeResponseBody(resp.Body, target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}
