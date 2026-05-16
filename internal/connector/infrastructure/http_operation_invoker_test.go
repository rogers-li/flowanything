package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

func TestHTTPOperationInvokerInvokeGET(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.String() != "http://orders.test/api/orders/o_123?verbose=true" {
				t.Fatalf("unexpected url %q", req.URL.String())
			}

			var body strings.Builder
			_ = json.NewEncoder(&body).Encode(map[string]any{"order_id": "o_123"})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body.String())),
			}, nil
		}),
	})

	result, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:      tenant.ID("tenant_1"),
		ID:            id.ID("connop_1"),
		Type:          connector.OperationTypeHTTP,
		BaseURL:       "http://orders.test/api",
		Method:        http.MethodGet,
		Path:          "/orders/{order_id}",
		TimeoutMillis: 1000,
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_1"),
		Args: map[string]any{
			"order_id": "o_123",
			"verbose":  true,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Data["order_id"] != "o_123" {
		t.Fatalf("unexpected data %#v", result.Data)
	}
}

func TestHTTPOperationInvokerInvokePOSTWithBearerAuth(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "tvly-test-token")

	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.String() != "https://api.tavily.com/search" {
				t.Fatalf("unexpected url %q", req.URL.String())
			}
			if req.Header.Get("Authorization") != "Bearer tvly-test-token" {
				t.Fatalf("unexpected authorization header %q", req.Header.Get("Authorization"))
			}
			if req.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("unexpected content type %q", req.Header.Get("Content-Type"))
			}

			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if body["query"] != "latest ai news" || body["max_results"].(float64) != 3 {
				t.Fatalf("unexpected request body %#v", body)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"query":"latest ai news","results":[]}`)),
			}, nil
		}),
	})

	result, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:      tenant.ID("tenant_1"),
		ID:            id.ID("connop_tavily_search"),
		Type:          connector.OperationTypeHTTP,
		BaseURL:       "https://api.tavily.com",
		Method:        http.MethodPost,
		Path:          "/search",
		TimeoutMillis: 1000,
		Auth: connector.AuthConfig{
			Type:      connector.AuthTypeBearer,
			SecretRef: "env:TAVILY_API_KEY",
		},
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_tavily_search"),
		Args: map[string]any{
			"query":       "latest ai news",
			"max_results": 3,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Data["query"] != "latest ai news" {
		t.Fatalf("unexpected data %#v", result.Data)
	}
}

func TestHTTPOperationInvokerSupportsAPIKeyQueryAuth(t *testing.T) {
	t.Setenv("JUHE_NEWS_API_KEY", "juhe-test-token")

	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/toutiao/index" {
				t.Fatalf("unexpected path %q", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("key") != "juhe-test-token" {
				t.Fatalf("expected api key in query, got %q", query.Get("key"))
			}
			if query.Get("type") != "keji" || query.Get("page_size") != "3" {
				t.Fatalf("unexpected query %q", req.URL.RawQuery)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"reason":"success","result":{"data":[]},"error_code":0}`)),
			}, nil
		}),
	})

	result, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:      tenant.ID("tenant_1"),
		ID:            id.ID("connop_juhe_news_list"),
		Type:          connector.OperationTypeHTTP,
		BaseURL:       "https://v.juhe.cn",
		Method:        http.MethodGet,
		Path:          "/toutiao/index",
		TimeoutMillis: 1000,
		Auth: connector.AuthConfig{
			Type:       connector.AuthTypeAPIKey,
			HeaderName: "query:key",
			SecretRef:  "env:JUHE_NEWS_API_KEY",
		},
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_juhe_news_list"),
		Args: map[string]any{
			"type":      "keji",
			"page_size": 3,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Data["error_code"].(float64) != 0 {
		t.Fatalf("unexpected data %#v", result.Data)
	}
}

func TestHTTPOperationInvokerSupportsPOSTPathQueryTemplate(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			if req.URL.Path != "/open-apis/docx/v1/documents/doc_1/blocks/doc_1/descendant" {
				t.Fatalf("unexpected path %q", req.URL.Path)
			}
			if req.URL.Query().Get("document_revision_id") != "-1" {
				t.Fatalf("unexpected query %q", req.URL.RawQuery)
			}

			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if _, ok := body["document_id"]; ok {
				t.Fatalf("path placeholder args must not remain in body: %#v", body)
			}
			if body["index"].(float64) != 0 {
				t.Fatalf("unexpected body %#v", body)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"code":0,"data":{"children":[]}}`)),
			}, nil
		}),
	})

	result, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:      tenant.ID("tenant_1"),
		ID:            id.ID("connop_feishu_create_nested_blocks"),
		Type:          connector.OperationTypeHTTP,
		BaseURL:       "https://open.feishu.cn",
		Method:        http.MethodPost,
		Path:          "/open-apis/docx/v1/documents/{document_id}/blocks/{block_id}/descendant?document_revision_id=-1",
		TimeoutMillis: 1000,
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_feishu_create_nested_blocks"),
		Args: map[string]any{
			"document_id": "doc_1",
			"block_id":    "doc_1",
			"index":       0,
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestHTTPOperationInvokerSupportsFeishuOAuth2UserAccessToken(t *testing.T) {
	previousProvider := defaultFeishuUserTokenProvider
	defaultFeishuUserTokenProvider = &feishuUserTokenProvider{}
	t.Cleanup(func() {
		defaultFeishuUserTokenProvider = previousProvider
	})
	t.Setenv("FEISHU_APP_ID", "cli_test")
	t.Setenv("FEISHU_APP_SECRET", "secret_test")
	t.Setenv("FEISHU_REFRESH_TOKEN", "refresh_1")

	authRequestCount := 0
	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/open-apis/auth/v3/app_access_token/internal":
				authRequestCount++
				if req.Method != http.MethodPost {
					t.Fatalf("expected app token POST, got %s", req.Method)
				}
				var body map[string]string
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode app token body: %v", err)
				}
				if body["app_id"] != "cli_test" || body["app_secret"] != "secret_test" {
					t.Fatalf("unexpected app token body %#v", body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"app_access_token":"app_token","expire":7200}`)),
				}, nil
			case "/open-apis/authen/v1/refresh_access_token":
				authRequestCount++
				if req.Header.Get("Authorization") != "Bearer app_token" {
					t.Fatalf("unexpected refresh authorization %q", req.Header.Get("Authorization"))
				}
				var body map[string]string
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode refresh token body: %v", err)
				}
				if body["grant_type"] != "refresh_token" || body["refresh_token"] != "refresh_1" {
					t.Fatalf("unexpected refresh token body %#v", body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"data":{"access_token":"user_token","refresh_token":"refresh_2","expires_in":7200}}`)),
				}, nil
			case "/open-apis/docx/v1/documents":
				if req.Header.Get("Authorization") != "Bearer user_token" {
					t.Fatalf("unexpected operation authorization %q", req.Header.Get("Authorization"))
				}
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode operation body: %v", err)
				}
				if body["title"] != "AI Report" {
					t.Fatalf("unexpected operation body %#v", body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"data":{"document":{"document_id":"doc_1"}}}`)),
				}, nil
			default:
				t.Fatalf("unexpected request path %q", req.URL.Path)
				return nil, nil
			}
		}),
	})

	result, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:      tenant.ID("tenant_1"),
		ID:            id.ID("connop_feishu_create_document"),
		Type:          connector.OperationTypeHTTP,
		BaseURL:       "https://open.feishu.cn",
		Method:        http.MethodPost,
		Path:          "/open-apis/docx/v1/documents",
		TimeoutMillis: 1000,
		Auth: connector.AuthConfig{
			Type:     connector.AuthTypeOAuth2,
			Provider: "feishu_user_access_token",
		},
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_feishu_create_document"),
		Args:        map[string]any{"title": "AI Report"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if authRequestCount != 2 {
		t.Fatalf("expected two auth requests, got %d", authRequestCount)
	}
}

func TestHTTPOperationInvokerSupportsFeishuOAuth2TenantAccessToken(t *testing.T) {
	previousProvider := defaultFeishuTenantTokenProvider
	defaultFeishuTenantTokenProvider = &feishuTenantTokenProvider{}
	t.Cleanup(func() {
		defaultFeishuTenantTokenProvider = previousProvider
	})
	t.Setenv("FEISHU_APP_ID", "cli_test")
	t.Setenv("FEISHU_APP_SECRET", "secret_test")

	tokenRequestCount := 0
	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/open-apis/auth/v3/tenant_access_token/internal":
				tokenRequestCount++
				var body map[string]string
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode tenant token body: %v", err)
				}
				if body["app_id"] != "cli_test" || body["app_secret"] != "secret_test" {
					t.Fatalf("unexpected tenant token body %#v", body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"tenant_access_token":"tenant_token","expire":7200}`)),
				}, nil
			case "/open-apis/docx/v1/documents":
				if req.Header.Get("Authorization") != "Bearer tenant_token" {
					t.Fatalf("unexpected operation authorization %q", req.Header.Get("Authorization"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"data":{"document":{"document_id":"doc_1"}}}`)),
				}, nil
			default:
				t.Fatalf("unexpected request path %q", req.URL.Path)
				return nil, nil
			}
		}),
	})

	spec := connector.OperationSpec{
		TenantID:      tenant.ID("tenant_1"),
		ID:            id.ID("connop_feishu_create_document"),
		Type:          connector.OperationTypeHTTP,
		BaseURL:       "https://open.feishu.cn",
		Method:        http.MethodPost,
		Path:          "/open-apis/docx/v1/documents",
		TimeoutMillis: 1000,
		Auth: connector.AuthConfig{
			Type:     connector.AuthTypeOAuth2,
			Provider: "feishu_tenant_access_token",
		},
	}
	for n := 0; n < 2; n++ {
		result, err := invoker.Invoke(context.Background(), spec, connector.InvokeRequest{
			ID:          id.ID("req_1"),
			TenantID:    tenant.ID("tenant_1"),
			OperationID: id.ID("connop_feishu_create_document"),
			Args:        map[string]any{"title": "AI Report"},
		})
		if err != nil {
			t.Fatalf("Invoke() error = %v", err)
		}
		if !result.Success {
			t.Fatal("expected success")
		}
	}
	if tokenRequestCount != 1 {
		t.Fatalf("expected tenant token to be cached, got %d token requests", tokenRequestCount)
	}
}

func TestHTTPOperationInvokerRefreshesFeishuTenantTokenOnInvalidAccessToken(t *testing.T) {
	previousProvider := defaultFeishuTenantTokenProvider
	defaultFeishuTenantTokenProvider = &feishuTenantTokenProvider{}
	t.Cleanup(func() {
		defaultFeishuTenantTokenProvider = previousProvider
	})
	t.Setenv("FEISHU_APP_ID", "cli_test")
	t.Setenv("FEISHU_APP_SECRET", "secret_test")

	tokenRequestCount := 0
	operationRequestCount := 0
	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/open-apis/auth/v3/tenant_access_token/internal":
				tokenRequestCount++
				token := "expired_token"
				if tokenRequestCount > 1 {
					token = "fresh_token"
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"tenant_access_token":"` + token + `","expire":7200}`)),
				}, nil
			case "/open-apis/docx/v1/documents":
				operationRequestCount++
				if operationRequestCount == 1 {
					if req.Header.Get("Authorization") != "Bearer expired_token" {
						t.Fatalf("unexpected first authorization %q", req.Header.Get("Authorization"))
					}
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Header:     http.Header{"Content-Type": []string{"application/json"}},
						Body:       io.NopCloser(strings.NewReader(`{"code":99991663,"msg":"Invalid access token for authorization. Please make a request with token attached."}`)),
					}, nil
				}
				if req.Header.Get("Authorization") != "Bearer fresh_token" {
					t.Fatalf("unexpected retry authorization %q", req.Header.Get("Authorization"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"code":0,"data":{"document":{"document_id":"doc_1"}}}`)),
				}, nil
			default:
				t.Fatalf("unexpected request path %q", req.URL.Path)
				return nil, nil
			}
		}),
	})

	result, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:      tenant.ID("tenant_1"),
		ID:            id.ID("connop_feishu_create_document"),
		Type:          connector.OperationTypeHTTP,
		BaseURL:       "https://open.feishu.cn",
		Method:        http.MethodPost,
		Path:          "/open-apis/docx/v1/documents",
		TimeoutMillis: 1000,
		Auth: connector.AuthConfig{
			Type:     connector.AuthTypeOAuth2,
			Provider: "feishu_tenant_access_token",
		},
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_feishu_create_document"),
		Args:        map[string]any{"title": "AI Report"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected retry to succeed")
	}
	if tokenRequestCount != 2 {
		t.Fatalf("expected token refresh after invalid token, got %d token requests", tokenRequestCount)
	}
	if operationRequestCount != 2 {
		t.Fatalf("expected operation retry, got %d requests", operationRequestCount)
	}
}

func TestHTTPOperationInvokerRejectsMissingBearerSecret(t *testing.T) {
	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("missing auth secret must fail before sending an external http request")
			return nil, nil
		}),
	})

	_, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID: tenant.ID("tenant_1"),
		ID:       id.ID("connop_tavily_search"),
		Type:     connector.OperationTypeHTTP,
		BaseURL:  "https://api.tavily.com",
		Method:   http.MethodPost,
		Path:     "/search",
		Auth: connector.AuthConfig{
			Type:      connector.AuthTypeBearer,
			SecretRef: "env:MISSING_TAVILY_API_KEY",
		},
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_tavily_search"),
		Args:        map[string]any{"query": "latest ai news"},
	})
	if err == nil {
		t.Fatal("expected missing secret error")
	}
}

func TestHTTPOperationInvokerSupportsMockMode(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPOperationInvokerWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("mock mode must not issue an external http request")
			return nil, nil
		}),
	})

	result, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:           tenant.ID("tenant_1"),
		ID:                 id.ID("connop_mock"),
		Type:               connector.OperationTypeHTTP,
		ImplementationMode: connector.ImplementationModeMock,
		BaseURL:            "http://orders.test",
		Method:             http.MethodPost,
		Path:               "/orders",
		TimeoutMillis:      1000,
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_mock"),
		Args:        map[string]any{"order_id": "o_123"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Success {
		t.Fatal("expected mock invocation success")
	}
	if result.Data["mode"] != string(connector.ImplementationModeMock) {
		t.Fatalf("unexpected mock result %#v", result.Data)
	}
}

func TestHTTPOperationInvokerRejectsUnimplementedMode(t *testing.T) {
	t.Parallel()

	invoker := NewHTTPOperationInvoker()
	_, err := invoker.Invoke(context.Background(), connector.OperationSpec{
		TenantID:           tenant.ID("tenant_1"),
		ID:                 id.ID("connop_template"),
		Type:               connector.OperationTypeHTTP,
		ImplementationMode: connector.ImplementationModeTemplateMapping,
		BaseURL:            "http://orders.test",
		Method:             http.MethodPost,
		Path:               "/orders",
		TimeoutMillis:      1000,
	}, connector.InvokeRequest{
		ID:          id.ID("req_1"),
		TenantID:    tenant.ID("tenant_1"),
		OperationID: id.ID("connop_template"),
	})
	if err == nil {
		t.Fatal("expected unimplemented implementation mode error")
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
