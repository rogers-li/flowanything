package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"flow-anything/internal/platform/contracts/connector"
	apperrors "flow-anything/internal/platform/kernel/errors"
)

type HTTPOperationInvoker struct {
	client *http.Client
}

func NewHTTPOperationInvoker() *HTTPOperationInvoker {
	return NewHTTPOperationInvokerWithClient(http.DefaultClient)
}

func NewHTTPOperationInvokerWithClient(client *http.Client) *HTTPOperationInvoker {
	if client == nil {
		client = http.DefaultClient
	}

	return &HTTPOperationInvoker{client: client}
}

// Invoke executes a configured HTTP connector operation.
//
// It performs path placeholder binding, query/body mapping, timeout isolation,
// and response normalization so Agent Runtime does not need to understand
// provider-specific HTTP details.
func (i *HTTPOperationInvoker) Invoke(ctx context.Context, operation connector.OperationSpec, req connector.InvokeRequest) (connector.InvokeResult, error) {
	if operation.Type != connector.OperationTypeHTTP {
		return connector.InvokeResult{}, apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector operation type")
	}
	mode := operation.ImplementationMode
	if mode == "" {
		mode = connector.ImplementationModeSimpleHTTP
	}
	switch mode {
	case connector.ImplementationModeSimpleHTTP:
	case connector.ImplementationModeMock:
		return connector.InvokeResult{
			RequestID: req.ID,
			Success:   true,
			Data: map[string]any{
				"operation_id": operation.ID.String(),
				"mode":         string(mode),
				"args":         req.Args,
			},
			FinishedAt: time.Now().UTC(),
		}, nil
	default:
		return connector.InvokeResult{}, apperrors.New(
			apperrors.CodeUnavailable,
			fmt.Sprintf("connector implementation mode %s is not implemented", mode),
		)
	}

	timeout := time.Duration(operation.TimeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	targetURL, remainingArgs, err := buildURL(operation, req.Args)
	if err != nil {
		return connector.InvokeResult{}, err
	}

	var payload []byte
	if methodAllowsBody(operation.Method) {
		payload, err = json.Marshal(remainingArgs)
		if err != nil {
			return connector.InvokeResult{}, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to encode connector request body", err)
		}
	}

	resp, err := i.doRequest(ctx, operation, targetURL, payload, timeout)
	if err != nil {
		return connector.InvokeResult{}, apperrors.Wrap(apperrors.CodeUnavailable, "connector http request failed", err)
	}
	data, statusCode, decodeErr := decodeAndCloseResponse(resp)
	if decodeErr != nil {
		return connector.InvokeResult{}, decodeErr
	}
	if shouldRetryOAuth2Auth(operation.Auth, statusCode, data) {
		resetOAuth2TokenCache(operation.Auth)
		resp, err = i.doRequest(ctx, operation, targetURL, payload, timeout)
		if err != nil {
			return connector.InvokeResult{}, apperrors.Wrap(apperrors.CodeUnavailable, "connector http request failed", err)
		}
		data, statusCode, decodeErr = decodeAndCloseResponse(resp)
		if decodeErr != nil {
			return connector.InvokeResult{}, decodeErr
		}
	}
	result := connector.InvokeResult{
		RequestID:  req.ID,
		Success:    statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices,
		Data:       data,
		FinishedAt: time.Now().UTC(),
	}
	if !result.Success {
		result.ErrorCode = fmt.Sprintf("http_status_%d", statusCode)
	}

	return result, nil
}

func (i *HTTPOperationInvoker) doRequest(ctx context.Context, operation connector.OperationSpec, targetURL string, payload []byte, timeout time.Duration) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	httpReq, err := http.NewRequestWithContext(ctx, strings.ToUpper(operation.Method), targetURL, body)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to build connector request", err)
	}
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	for key, value := range operation.Headers {
		httpReq.Header.Set(key, value)
	}
	client := *i.client
	client.Timeout = timeout
	if err := applyAuth(ctx, &client, httpReq, operation.Auth); err != nil {
		return nil, err
	}
	return client.Do(httpReq)
}

func applyAuth(ctx context.Context, client *http.Client, req *http.Request, auth connector.AuthConfig) error {
	switch auth.Type {
	case "", connector.AuthTypeNone:
		return nil
	case connector.AuthTypeBearer:
		token, err := resolveSecret(auth.SecretRef)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	case connector.AuthTypeAPIKey:
		headerName := strings.TrimSpace(auth.HeaderName)
		if headerName == "" {
			return apperrors.New(apperrors.CodeInvalidArgument, "api_key auth requires header_name")
		}
		token, err := resolveSecret(auth.SecretRef)
		if err != nil {
			return err
		}
		if queryName, ok := apiKeyQueryName(headerName); ok {
			query := req.URL.Query()
			query.Set(queryName, token)
			req.URL.RawQuery = query.Encode()
			return nil
		}
		req.Header.Set(headerName, token)
		return nil
	case connector.AuthTypeBasic:
		token, err := resolveSecret(auth.SecretRef)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Basic "+token)
		return nil
	case connector.AuthTypeOAuth2:
		token, err := resolveOAuth2Token(ctx, client, auth)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	default:
		return apperrors.New(apperrors.CodeInvalidArgument, "unsupported connector auth type")
	}
}

func apiKeyQueryName(headerName string) (string, bool) {
	for _, prefix := range []string{"query:", "query.", "param:", "param."} {
		if strings.HasPrefix(headerName, prefix) {
			name := strings.TrimSpace(strings.TrimPrefix(headerName, prefix))
			return name, name != ""
		}
	}
	return "", false
}

func resolveOAuth2Token(ctx context.Context, client *http.Client, auth connector.AuthConfig) (string, error) {
	switch strings.ToLower(strings.TrimSpace(auth.Provider)) {
	case "feishu", "feishu_user_access_token", "lark", "lark_user_access_token":
		return defaultFeishuUserTokenProvider.Token(ctx, client, auth)
	case "feishu_tenant_access_token", "lark_tenant_access_token":
		return defaultFeishuTenantTokenProvider.Token(ctx, client, auth)
	case "":
		if auth.SecretRef != "" {
			return resolveSecret(auth.SecretRef)
		}
		return "", apperrors.New(apperrors.CodeInvalidArgument, "oauth2 connector auth requires provider or secret_ref")
	default:
		return "", apperrors.New(
			apperrors.CodeUnavailable,
			fmt.Sprintf("oauth2 connector auth provider %s is not implemented", auth.Provider),
		)
	}
}

const (
	defaultFeishuAppAccessTokenURL = "https://open.feishu.cn/open-apis/auth/v3/app_access_token/internal"
	defaultFeishuTenantTokenURL    = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	defaultFeishuAccessTokenURL    = "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token"
	defaultFeishuRefreshTokenURL   = "https://open.feishu.cn/open-apis/authen/v1/refresh_access_token"
)

var (
	defaultFeishuTenantTokenProvider = &feishuTenantTokenProvider{}
	defaultFeishuUserTokenProvider   = &feishuUserTokenProvider{}
)

type feishuTenantTokenProvider struct {
	mu          sync.Mutex
	cacheKey    string
	accessToken string
	expiresAt   time.Time
}

func (p *feishuTenantTokenProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheKey = ""
	p.accessToken = ""
	p.expiresAt = time.Time{}
}

type feishuTenantTokenConfig struct {
	AppID     string
	AppSecret string
	TokenURL  string
	CacheKey  string
}

// Token returns a Feishu tenant_access_token derived from app_id/app_secret.
//
// This is the default mode for server-side Docx automation because it does not
// require a user OAuth authorization code or refresh token.
func (p *feishuTenantTokenProvider) Token(ctx context.Context, client *http.Client, auth connector.AuthConfig) (string, error) {
	cfg, err := buildFeishuTenantTokenConfig(auth)
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
		return "", apperrors.New(apperrors.CodeUnavailable, "feishu tenant_access_token response is empty")
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
	return p.accessToken, nil
}

func buildFeishuTenantTokenConfig(auth connector.AuthConfig) (feishuTenantTokenConfig, error) {
	appID, err := resolveRequiredSecret("feishu app id", auth.ClientIDRef, "env:FEISHU_APP_ID")
	if err != nil {
		return feishuTenantTokenConfig{}, err
	}
	appSecret, err := resolveRequiredSecret("feishu app secret", auth.ClientSecretRef, "env:FEISHU_APP_SECRET")
	if err != nil {
		return feishuTenantTokenConfig{}, err
	}
	tokenURL := strings.TrimSpace(auth.TenantTokenURL)
	if tokenURL == "" {
		tokenURL = strings.TrimSpace(auth.AccessTokenURL)
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
	var resp feishuTenantTokenResponse
	err := postJSON(ctx, client, cfg.TokenURL, nil, map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	}, &resp)
	if err != nil {
		return "", 0, err
	}
	if resp.Code != 0 {
		return "", 0, apperrors.New(apperrors.CodeUnavailable, fmt.Sprintf("feishu tenant_access_token failed: %s", feishuErrorMessage(resp.Msg, resp.Message)))
	}
	token := resp.TenantAccessToken
	if token == "" {
		token = resp.Data.TenantAccessToken
	}
	expire := resp.Expire
	if expire <= 0 {
		expire = resp.Data.Expire
	}
	return token, expire, nil
}

type feishuUserTokenProvider struct {
	mu           sync.Mutex
	cacheKey     string
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

func (p *feishuUserTokenProvider) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheKey = ""
	p.accessToken = ""
	p.refreshToken = ""
	p.expiresAt = time.Time{}
}

type feishuUserTokenConfig struct {
	AppID             string
	AppSecret         string
	RefreshToken      string
	AuthorizationCode string
	AppAccessTokenURL string
	AccessTokenURL    string
	RefreshTokenURL   string
	CacheKey          string
}

// Token returns a Feishu user_access_token suitable for Docx APIs.
//
// Feishu user_access_token is short lived, so the provider exchanges an
// authorization code once or refreshes from a refresh_token, then caches the
// access token in memory until shortly before expiry.
func (p *feishuUserTokenProvider) Token(ctx context.Context, client *http.Client, auth connector.AuthConfig) (string, error) {
	cfg, staticToken, err := buildFeishuUserTokenConfig(auth)
	if err != nil {
		return "", err
	}
	if staticToken != "" && cfg.RefreshToken == "" && cfg.AuthorizationCode == "" {
		return staticToken, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cacheKey == cfg.CacheKey && p.accessToken != "" && time.Now().Before(p.expiresAt) {
		return p.accessToken, nil
	}
	if p.cacheKey == cfg.CacheKey && p.refreshToken != "" {
		cfg.RefreshToken = p.refreshToken
	}

	appAccessToken, err := fetchFeishuAppAccessToken(ctx, client, cfg)
	if err != nil {
		return "", err
	}

	var token feishuUserToken
	if cfg.RefreshToken != "" {
		token, err = refreshFeishuUserAccessToken(ctx, client, cfg, appAccessToken)
	} else {
		token, err = exchangeFeishuAuthorizationCode(ctx, client, cfg, appAccessToken)
	}
	if err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", apperrors.New(apperrors.CodeUnavailable, "feishu access_token response is empty")
	}

	refreshToken := token.RefreshToken
	if refreshToken == "" {
		refreshToken = cfg.RefreshToken
	}
	expiresIn := token.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	if expiresIn > 120 {
		expiresAt = expiresAt.Add(-60 * time.Second)
	}

	p.cacheKey = cfg.CacheKey
	p.accessToken = token.AccessToken
	p.refreshToken = refreshToken
	p.expiresAt = expiresAt
	return p.accessToken, nil
}

func buildFeishuUserTokenConfig(auth connector.AuthConfig) (feishuUserTokenConfig, string, error) {
	refreshToken, hasRefresh, err := resolveFirstAvailableSecret(auth.RefreshTokenRef, "env:FEISHU_REFRESH_TOKEN")
	if err != nil {
		return feishuUserTokenConfig{}, "", err
	}
	authCode, hasAuthCode, err := resolveFirstAvailableSecret(auth.AuthorizationCodeRef, "env:FEISHU_AUTHORIZATION_CODE", "env:FEISHU_AUTH_CODE")
	if err != nil {
		return feishuUserTokenConfig{}, "", err
	}
	staticToken, hasStaticToken, err := resolveFirstAvailableSecret(auth.SecretRef, "env:FEISHU_USER_ACCESS_TOKEN")
	if err != nil {
		return feishuUserTokenConfig{}, "", err
	}
	if !hasRefresh && !hasAuthCode {
		if hasStaticToken {
			return feishuUserTokenConfig{}, staticToken, nil
		}
		return feishuUserTokenConfig{}, "", apperrors.New(
			apperrors.CodeUnavailable,
			"feishu oauth2 auth requires FEISHU_REFRESH_TOKEN or FEISHU_AUTHORIZATION_CODE",
		)
	}

	appID, err := resolveRequiredSecret("feishu app id", auth.ClientIDRef, "env:FEISHU_APP_ID")
	if err != nil {
		return feishuUserTokenConfig{}, "", err
	}
	appSecret, err := resolveRequiredSecret("feishu app secret", auth.ClientSecretRef, "env:FEISHU_APP_SECRET")
	if err != nil {
		return feishuUserTokenConfig{}, "", err
	}

	appAccessTokenURL := strings.TrimSpace(auth.AppAccessTokenURL)
	if appAccessTokenURL == "" {
		appAccessTokenURL = defaultFeishuAppAccessTokenURL
	}
	accessTokenURL := strings.TrimSpace(auth.AccessTokenURL)
	if accessTokenURL == "" {
		accessTokenURL = defaultFeishuAccessTokenURL
	}
	refreshTokenURL := strings.TrimSpace(auth.RefreshTokenURL)
	if refreshTokenURL == "" {
		refreshTokenURL = defaultFeishuRefreshTokenURL
	}

	cacheKey := strings.Join([]string{
		appID,
		refreshToken,
		authCode,
		appAccessTokenURL,
		accessTokenURL,
		refreshTokenURL,
	}, "|")
	return feishuUserTokenConfig{
		AppID:             appID,
		AppSecret:         appSecret,
		RefreshToken:      refreshToken,
		AuthorizationCode: authCode,
		AppAccessTokenURL: appAccessTokenURL,
		AccessTokenURL:    accessTokenURL,
		RefreshTokenURL:   refreshTokenURL,
		CacheKey:          cacheKey,
	}, "", nil
}

type feishuAppAccessTokenResponse struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	Message        string `json:"message"`
	AppAccessToken string `json:"app_access_token"`
	Expire         int    `json:"expire"`
	Data           struct {
		AppAccessToken string `json:"app_access_token"`
		Expire         int    `json:"expire"`
	} `json:"data"`
}

type feishuUserToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

type feishuUserTokenResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Data    struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		TokenType        string `json:"token_type"`
		ExpiresIn        int    `json:"expires_in"`
		RefreshExpiresIn int    `json:"refresh_expires_in"`
		Scope            string `json:"scope"`
	} `json:"data"`
}

func fetchFeishuAppAccessToken(ctx context.Context, client *http.Client, cfg feishuUserTokenConfig) (string, error) {
	var resp feishuAppAccessTokenResponse
	err := postJSON(ctx, client, cfg.AppAccessTokenURL, nil, map[string]string{
		"app_id":     cfg.AppID,
		"app_secret": cfg.AppSecret,
	}, &resp)
	if err != nil {
		return "", err
	}
	if resp.Code != 0 {
		return "", apperrors.New(apperrors.CodeUnavailable, fmt.Sprintf("feishu app_access_token failed: %s", feishuErrorMessage(resp.Msg, resp.Message)))
	}
	token := resp.AppAccessToken
	if token == "" {
		token = resp.Data.AppAccessToken
	}
	if token == "" {
		return "", apperrors.New(apperrors.CodeUnavailable, "feishu app_access_token response is empty")
	}
	return token, nil
}

func refreshFeishuUserAccessToken(ctx context.Context, client *http.Client, cfg feishuUserTokenConfig, appAccessToken string) (feishuUserToken, error) {
	var resp feishuUserTokenResponse
	err := postJSON(ctx, client, cfg.RefreshTokenURL, map[string]string{
		"Authorization": "Bearer " + appAccessToken,
	}, map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": cfg.RefreshToken,
	}, &resp)
	if err != nil {
		return feishuUserToken{}, err
	}
	return parseFeishuUserTokenResponse(resp)
}

func exchangeFeishuAuthorizationCode(ctx context.Context, client *http.Client, cfg feishuUserTokenConfig, appAccessToken string) (feishuUserToken, error) {
	var resp feishuUserTokenResponse
	err := postJSON(ctx, client, cfg.AccessTokenURL, map[string]string{
		"Authorization": "Bearer " + appAccessToken,
	}, map[string]string{
		"grant_type": "authorization_code",
		"code":       cfg.AuthorizationCode,
	}, &resp)
	if err != nil {
		return feishuUserToken{}, err
	}
	return parseFeishuUserTokenResponse(resp)
}

func parseFeishuUserTokenResponse(resp feishuUserTokenResponse) (feishuUserToken, error) {
	if resp.Code != 0 {
		return feishuUserToken{}, apperrors.New(apperrors.CodeUnavailable, fmt.Sprintf("feishu user_access_token failed: %s", feishuErrorMessage(resp.Msg, resp.Message)))
	}
	return feishuUserToken{
		AccessToken:  resp.Data.AccessToken,
		RefreshToken: resp.Data.RefreshToken,
		ExpiresIn:    resp.Data.ExpiresIn,
	}, nil
}

func postJSON(ctx context.Context, client *http.Client, targetURL string, headers map[string]string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to encode auth request body", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to build auth request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeUnavailable, "auth http request failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(resp.Body)
		return apperrors.New(
			apperrors.CodeUnavailable,
			fmt.Sprintf("auth http request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw))),
		)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return apperrors.Wrap(apperrors.CodeUnavailable, "failed to decode auth response", err)
	}
	return nil
}

func feishuErrorMessage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return "unknown error"
}

func resolveSecret(secretRef string) (string, error) {
	candidates, err := secretEnvCandidates(secretRef)
	if err != nil {
		return "", err
	}
	for _, key := range candidates {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value, nil
		}
	}
	return "", apperrors.New(
		apperrors.CodeUnavailable,
		fmt.Sprintf("connector auth secret is not set: %s", candidates[0]),
	)
}

func resolveRequiredSecret(label string, refs ...string) (string, error) {
	value, ok, err := resolveFirstAvailableSecret(refs...)
	if err != nil {
		return "", err
	}
	if ok {
		return value, nil
	}
	return "", apperrors.New(apperrors.CodeUnavailable, fmt.Sprintf("%s is not set", label))
}

func resolveFirstAvailableSecret(refs ...string) (string, bool, error) {
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		value, ok, err := lookupSecret(ref)
		if err != nil {
			return "", false, err
		}
		if ok {
			return value, true, nil
		}
	}
	return "", false, nil
}

func lookupSecret(secretRef string) (string, bool, error) {
	candidates, err := secretEnvCandidates(secretRef)
	if err != nil {
		return "", false, err
	}
	for _, key := range candidates {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value, true, nil
		}
	}
	return "", false, nil
}

func secretEnvCandidates(secretRef string) ([]string, error) {
	ref := strings.TrimSpace(secretRef)
	if ref == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "connector auth secret_ref is required")
	}
	if strings.HasPrefix(ref, "env:") {
		key := strings.TrimSpace(strings.TrimPrefix(ref, "env:"))
		if key == "" {
			return nil, apperrors.New(apperrors.CodeInvalidArgument, "connector auth env secret_ref is empty")
		}
		return []string{key}, nil
	}
	if strings.HasPrefix(ref, "$") {
		key := strings.TrimSpace(strings.TrimPrefix(ref, "$"))
		if key == "" {
			return nil, apperrors.New(apperrors.CodeInvalidArgument, "connector auth env secret_ref is empty")
		}
		return []string{key}, nil
	}

	normalized := normalizeSecretRefToEnvKey(ref)
	if normalized == ref {
		return []string{ref}, nil
	}
	return []string{ref, normalized}, nil
}

func normalizeSecretRefToEnvKey(secretRef string) string {
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range secretRef {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.ToUpper(strings.Trim(builder.String(), "_"))
}

func buildURL(operation connector.OperationSpec, args map[string]any) (string, map[string]any, error) {
	base, err := url.Parse(operation.BaseURL)
	if err != nil {
		return "", nil, apperrors.Wrap(apperrors.CodeInvalidArgument, "invalid connector base_url", err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return "", nil, apperrors.New(apperrors.CodeInvalidArgument, "connector base_url must use http or https")
	}

	path, rawQuery, _ := strings.Cut(operation.Path, "?")
	remaining := copyArgs(args)
	for key, value := range args {
		placeholder := "{" + key + "}"
		if strings.Contains(path, placeholder) {
			path = strings.ReplaceAll(path, placeholder, url.PathEscape(fmt.Sprint(value)))
			delete(remaining, key)
		}
		if strings.Contains(rawQuery, placeholder) {
			rawQuery = strings.ReplaceAll(rawQuery, placeholder, url.QueryEscape(fmt.Sprint(value)))
			delete(remaining, key)
		}
	}

	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(path, "/")
	if rawQuery != "" {
		base.RawQuery = rawQuery
	}
	if !methodAllowsBody(operation.Method) {
		query := base.Query()
		for key, value := range remaining {
			query.Set(key, fmt.Sprint(value))
		}
		base.RawQuery = query.Encode()
		remaining = map[string]any{}
	}

	return base.String(), remaining, nil
}

func copyArgs(args map[string]any) map[string]any {
	result := make(map[string]any, len(args))
	for key, value := range args {
		result[key] = value
	}
	return result
}

func methodAllowsBody(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func decodeResponse(resp *http.Response) (map[string]any, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to read connector response", err)
	}
	if len(body) == 0 {
		return map[string]any{"status_code": resp.StatusCode}, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err == nil {
		decoded["status_code"] = resp.StatusCode
		return decoded, nil
	}

	return map[string]any{
		"status_code": resp.StatusCode,
		"body":        string(body),
	}, nil
}

func decodeAndCloseResponse(resp *http.Response) (map[string]any, int, error) {
	defer resp.Body.Close()
	data, err := decodeResponse(resp)
	return data, resp.StatusCode, err
}

func shouldRetryOAuth2Auth(auth connector.AuthConfig, statusCode int, data map[string]any) bool {
	if auth.Type != connector.AuthTypeOAuth2 {
		return false
	}
	if statusCode != http.StatusBadRequest && statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden {
		return false
	}
	if intFromAny(data["code"]) == 99991663 {
		return true
	}
	messageParts := []string{
		stringFromAny(data["msg"]),
		stringFromAny(data["message"]),
		stringFromAny(data["body"]),
	}
	if nested, ok := data["error"].(map[string]any); ok {
		messageParts = append(messageParts, stringFromAny(nested["message"]))
	}
	message := strings.ToLower(strings.Join(messageParts, " "))
	return strings.Contains(message, "invalid access token") || strings.Contains(message, "token attached")
}

func resetOAuth2TokenCache(auth connector.AuthConfig) {
	switch strings.ToLower(strings.TrimSpace(auth.Provider)) {
	case "feishu_tenant_access_token", "lark_tenant_access_token":
		defaultFeishuTenantTokenProvider.Reset()
	case "feishu", "feishu_user_access_token", "lark", "lark_user_access_token":
		defaultFeishuUserTokenProvider.Reset()
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	default:
		return 0
	}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
