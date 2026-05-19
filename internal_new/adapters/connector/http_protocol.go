package connectoradapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"flow-anything/core/connector"
)

const HTTPProtocolKind = "http"

// HTTPProtocolExecutor executes connector operations over HTTP. It intentionally
// returns non-2xx responses as data instead of errors so workflows can branch on
// API failure payloads.
type HTTPProtocolExecutor struct {
	Client         *http.Client
	SecretResolver SecretResolver
}

func (e HTTPProtocolExecutor) Kind() string { return HTTPProtocolKind }

func (e HTTPProtocolExecutor) ValidateConnector(connector connector.ConnectorSpec) error {
	if connector.Protocol.BaseURL == "" {
		return fmt.Errorf("http connector %q requires protocol.base_url", connector.ID)
	}
	if _, err := url.Parse(connector.Protocol.BaseURL); err != nil {
		return fmt.Errorf("invalid connector base url: %w", err)
	}
	return nil
}

func (e HTTPProtocolExecutor) ValidateOperation(_ connector.ConnectorSpec, operation connector.OperationSpec) error {
	if operation.Request.Method == "" {
		return fmt.Errorf("operation %q requires request.method", operation.ID)
	}
	if operation.Request.Path == "" {
		return fmt.Errorf("operation %q requires request.path", operation.ID)
	}
	return nil
}

func (e HTTPProtocolExecutor) Execute(ctx context.Context, req connector.ProtocolRequest) (connector.ProtocolResult, error) {
	httpClient := e.Client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	targetURL, err := buildURL(req.Connector, req.Operation, req.Input)
	if err != nil {
		return connector.ProtocolResult{}, err
	}
	body, err := requestBody(req.Operation, req.Input)
	if err != nil {
		return connector.ProtocolResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, strings.ToUpper(req.Operation.Request.Method), targetURL, body)
	if err != nil {
		return connector.ProtocolResult{}, err
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for key, value := range req.Operation.Request.Headers {
		httpReq.Header.Set(key, value)
	}
	if err := applyAuth(httpReq, req.Connector, e.SecretResolver, httpClient); err != nil {
		return connector.ProtocolResult{}, err
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return connector.ProtocolResult{}, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return connector.ProtocolResult{}, err
	}
	parsedBody := parseBody(rawBody)
	success := statusIsSuccess(resp.StatusCode, req.Operation.Response.SuccessStatusCodes)
	output := map[string]any{
		"success":     success,
		"status_code": resp.StatusCode,
		"body":        parsedBody,
		"headers":     responseHeaders(resp.Header),
	}
	return connector.ProtocolResult{Output: output, Raw: string(rawBody)}, nil
}

func buildURL(connector connector.ConnectorSpec, operation connector.OperationSpec, input map[string]any) (string, error) {
	path := operation.Request.Path
	for _, placeholder := range pathPlaceholders(path) {
		inputField := operation.Request.PathParams[placeholder]
		if inputField == "" {
			inputField = placeholder
		}
		value, ok := input[inputField]
		if !ok {
			return "", fmt.Errorf("path param %q requires input field %q", placeholder, inputField)
		}
		path = strings.ReplaceAll(path, "{"+placeholder+"}", url.PathEscape(fmt.Sprint(value)))
	}
	resolved, err := url.Parse(strings.TrimRight(connector.Protocol.BaseURL, "/") + "/" + strings.TrimLeft(path, "/"))
	if err != nil {
		return "", err
	}
	query := resolved.Query()
	for key, value := range operation.Request.Query {
		query.Set(key, value)
	}
	for queryName, inputField := range operation.Request.QueryParams {
		value, ok := input[inputField]
		if !ok {
			continue
		}
		query.Set(queryName, fmt.Sprint(value))
	}
	resolved.RawQuery = query.Encode()
	return resolved.String(), nil
}

func requestBody(operation connector.OperationSpec, input map[string]any) (io.Reader, error) {
	method := strings.ToUpper(operation.Request.Method)
	if method == http.MethodGet || method == http.MethodDelete || method == http.MethodHead {
		return nil, nil
	}
	payload := any(input)
	if operation.Request.BodyField != "" {
		var ok bool
		payload, ok = input[operation.Request.BodyField]
		if !ok {
			return nil, fmt.Errorf("body_field %q is missing from input", operation.Request.BodyField)
		}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func pathPlaceholders(path string) []string {
	names := []string{}
	remaining := path
	for {
		start := strings.Index(remaining, "{")
		if start < 0 {
			return names
		}
		end := strings.Index(remaining[start+1:], "}")
		if end < 0 {
			return names
		}
		name := strings.TrimSpace(remaining[start+1 : start+1+end])
		if name != "" {
			names = append(names, name)
		}
		remaining = remaining[start+1+end+1:]
	}
}

func applyAuth(req *http.Request, connector connector.ConnectorSpec, resolver SecretResolver, client *http.Client) error {
	authType := strings.ToLower(strings.TrimSpace(connector.Auth.Type))
	if authType == "" || authType == "none" {
		return nil
	}
	switch authType {
	case "bearer":
		secret := resolveSecret(connector.Auth.SecretRef, resolver)
		if secret == "" {
			return fmt.Errorf("bearer auth requires secret")
		}
		req.Header.Set("Authorization", "Bearer "+secret)
	case "api_key":
		secret := resolveSecret(connector.Auth.SecretRef, resolver)
		if secret == "" {
			return fmt.Errorf("api_key auth requires secret")
		}
		name := stringFromMap(connector.Auth.Config, "name", "Authorization")
		location := strings.ToLower(stringFromMap(connector.Auth.Config, "in", "header"))
		switch location {
		case "query":
			query := req.URL.Query()
			query.Set(name, secret)
			req.URL.RawQuery = query.Encode()
		default:
			req.Header.Set(name, secret)
		}
	case "basic":
		username := stringFromMap(connector.Auth.Config, "username", "")
		if username == "" {
			return fmt.Errorf("basic auth requires config.username")
		}
		secret := resolveSecret(connector.Auth.SecretRef, resolver)
		req.SetBasicAuth(username, secret)
	case "oauth2":
		token, err := resolveOAuth2Token(req.Context(), client, connector.Auth, resolver)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	default:
		return fmt.Errorf("unsupported http connector auth type %q", connector.Auth.Type)
	}
	return nil
}

func resolveSecret(ref string, resolver SecretResolver) string {
	if resolver == nil {
		return strings.TrimSpace(ref)
	}
	if value, ok := resolver.ResolveSecret(ref); ok {
		return value
	}
	return strings.TrimSpace(ref)
}

func resolveOAuth2Token(ctx context.Context, client *http.Client, auth connector.AuthSpec, resolver SecretResolver) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(stringFromMap(auth.Config, "provider", "")))
	switch provider {
	case "feishu_tenant_access_token", "lark_tenant_access_token":
		return defaultFeishuTenantTokenProvider.Token(ctx, client, auth, resolver)
	case "":
		if token := resolveSecret(auth.SecretRef, resolver); token != "" {
			return token, nil
		}
		return "", fmt.Errorf("oauth2 connector auth requires provider or secret_ref")
	default:
		return "", fmt.Errorf("oauth2 connector auth provider %q is not implemented", provider)
	}
}

const defaultFeishuTenantTokenURL = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"

var defaultFeishuTenantTokenProvider = &feishuTenantTokenProvider{}

type feishuTenantTokenProvider struct {
	mu          sync.Mutex
	cacheKey    string
	accessToken string
	expiresAt   time.Time
}

type feishuTenantTokenConfig struct {
	AppID     string
	AppSecret string
	TokenURL  string
	CacheKey  string
}

func (p *feishuTenantTokenProvider) Token(ctx context.Context, client *http.Client, auth connector.AuthSpec, resolver SecretResolver) (string, error) {
	cfg, err := buildFeishuTenantTokenConfig(auth, resolver)
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cacheKey == cfg.CacheKey && p.accessToken != "" && time.Now().Before(p.expiresAt) {
		return p.accessToken, nil
	}

	token, expiresIn, err := fetchFeishuTenantAccessToken(ctx, client, cfg)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("feishu tenant_access_token response is empty")
	}
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	if expiresIn > 120 {
		expiresAt = expiresAt.Add(-60 * time.Second)
	}

	p.cacheKey = cfg.CacheKey
	p.accessToken = token
	p.expiresAt = expiresAt
	return token, nil
}

func buildFeishuTenantTokenConfig(auth connector.AuthSpec, resolver SecretResolver) (feishuTenantTokenConfig, error) {
	appIDRef := stringFromMap(auth.Config, "client_id_ref", "env:FEISHU_APP_ID")
	appSecretRef := stringFromMap(auth.Config, "client_secret_ref", "env:FEISHU_APP_SECRET")
	appID := resolveSecret(appIDRef, resolver)
	appSecret := resolveSecret(appSecretRef, resolver)
	if appID == "" {
		return feishuTenantTokenConfig{}, fmt.Errorf("feishu oauth2 auth requires client_id_ref")
	}
	if appSecret == "" {
		return feishuTenantTokenConfig{}, fmt.Errorf("feishu oauth2 auth requires client_secret_ref")
	}
	tokenURL := stringFromMap(auth.Config, "tenant_access_token_url", "")
	if tokenURL == "" {
		tokenURL = stringFromMap(auth.Config, "access_token_url", "")
	}
	if tokenURL == "" {
		tokenURL = defaultFeishuTenantTokenURL
	}
	return feishuTenantTokenConfig{
		AppID:     appID,
		AppSecret: appSecret,
		TokenURL:  tokenURL,
		CacheKey:  strings.Join([]string{appID, tokenURL}, "|"),
	}, nil
}

type feishuTenantTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	Message           string `json:"message"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
	Data              struct {
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	} `json:"data"`
}

func fetchFeishuTenantAccessToken(ctx context.Context, client *http.Client, cfg feishuTenantTokenConfig) (string, int, error) {
	payload, err := json.Marshal(map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	})
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, fmt.Errorf("feishu tenant_access_token http status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed feishuTenantTokenResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, err
	}
	if parsed.Code != 0 {
		return "", 0, fmt.Errorf("feishu tenant_access_token failed: %s", firstString(parsed.Msg, parsed.Message))
	}
	token := parsed.TenantAccessToken
	expire := parsed.Expire
	if token == "" {
		token = parsed.Data.TenantAccessToken
		expire = parsed.Data.Expire
	}
	return token, expire, nil
}

func parseBody(data []byte) any {
	if len(data) == 0 {
		return nil
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err == nil {
		return parsed
	}
	return string(data)
}

func statusIsSuccess(statusCode int, allowed []int) bool {
	if len(allowed) == 0 {
		return statusCode >= 200 && statusCode < 300
	}
	for _, status := range allowed {
		if statusCode == status {
			return true
		}
	}
	return false
}

func responseHeaders(headers http.Header) map[string]any {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		out[key] = headers.Values(key)
	}
	return out
}

func stringFromMap(source map[string]any, key string, fallback string) string {
	if source == nil {
		return fallback
	}
	value, ok := source[key]
	if !ok {
		return fallback
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return fallback
	}
	return text
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
