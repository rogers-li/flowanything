package connectoradapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"flow-anything/core/connector"
)

func TestHTTPProtocolExecutorReturnsHTTPErrorAsOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "agent" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "bad_request"})
	}))
	defer server.Close()

	executor := HTTPProtocolExecutor{}
	result, err := executor.Execute(context.Background(), connector.ProtocolRequest{
		Connector: connector.ConnectorSpec{
			ID:       "conn_search",
			Protocol: connector.ProtocolSpec{Kind: HTTPProtocolKind, BaseURL: server.URL + "/api"},
		},
		Operation: connector.OperationSpec{
			ID: "op_search",
			Request: connector.OperationRequest{
				Method:      "GET",
				Path:        "/search",
				QueryParams: map[string]string{"q": "q"},
			},
		},
		Input: map[string]any{"q": "agent"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["success"] != false {
		t.Fatalf("expected success=false, got %#v", result.Output["success"])
	}
	if result.Output["status_code"] != http.StatusBadRequest {
		t.Fatalf("unexpected status: %#v", result.Output["status_code"])
	}
}

func TestHTTPProtocolExecutorAppliesAPIKeyToQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "juhe-token" {
			t.Fatalf("expected api key in query, got raw query %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"error_code": 0})
	}))
	defer server.Close()

	executor := HTTPProtocolExecutor{
		Client: server.Client(),
		SecretResolver: mapSecretResolver{
			"env:JUHE_NEWS_API_KEY": "juhe-token",
		},
	}
	result, err := executor.Execute(context.Background(), connector.ProtocolRequest{
		Connector: connector.ConnectorSpec{
			ID:       "conn_juhe_news",
			Protocol: connector.ProtocolSpec{Kind: HTTPProtocolKind, BaseURL: server.URL},
			Auth: connector.AuthSpec{
				Type:      "api_key",
				SecretRef: "env:JUHE_NEWS_API_KEY",
				Config: map[string]any{
					"in":   "query",
					"name": "key",
				},
			},
		},
		Operation: connector.OperationSpec{
			ID: "connop_juhe_news_list",
			Request: connector.OperationRequest{
				Method: "GET",
				Path:   "/toutiao/index",
			},
		},
		Input: map[string]any{"type": "keji"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["success"] != true {
		t.Fatalf("expected success=true, got %#v", result.Output)
	}
}

func TestHTTPProtocolExecutorSkipsMissingOptionalQueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("type") != "keji" {
			t.Fatalf("expected mapped type query, got %q", r.URL.RawQuery)
		}
		if query.Has("is_filter") {
			t.Fatalf("missing optional input should not create query param: %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"error_code": 0})
	}))
	defer server.Close()

	executor := HTTPProtocolExecutor{Client: server.Client()}
	result, err := executor.Execute(context.Background(), connector.ProtocolRequest{
		Connector: connector.ConnectorSpec{
			ID:       "conn_juhe_news",
			Protocol: connector.ProtocolSpec{Kind: HTTPProtocolKind, BaseURL: server.URL},
		},
		Operation: connector.OperationSpec{
			ID: "connop_juhe_news_list",
			Request: connector.OperationRequest{
				Method: "GET",
				Path:   "/toutiao/index",
				QueryParams: map[string]string{
					"is_filter": "is_filter",
					"type":      "type",
				},
			},
		},
		Input: map[string]any{"type": "keji"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["success"] != true {
		t.Fatalf("expected success=true, got %#v", result.Output)
	}
}

func TestHTTPProtocolExecutorUsesFeishuTenantOAuth2Token(t *testing.T) {
	defaultFeishuTenantTokenProvider = &feishuTenantTokenProvider{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode token payload: %v", err)
			}
			if payload["app_id"] != "app-id" || payload["app_secret"] != "app-secret" {
				t.Fatalf("unexpected token payload: %#v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":                0,
				"tenant_access_token": "tenant-token",
				"expire":              7200,
			})
		case "/open-apis/docx/v1/documents":
			if got := r.Header.Get("Authorization"); got != "Bearer tenant-token" {
				t.Fatalf("unexpected auth header: %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	executor := HTTPProtocolExecutor{
		Client: server.Client(),
		SecretResolver: mapSecretResolver{
			"env:FEISHU_APP_ID":     "app-id",
			"env:FEISHU_APP_SECRET": "app-secret",
		},
	}
	result, err := executor.Execute(context.Background(), connector.ProtocolRequest{
		Connector: connector.ConnectorSpec{
			ID:       "conn_feishu_doc",
			Protocol: connector.ProtocolSpec{Kind: HTTPProtocolKind, BaseURL: server.URL},
			Auth: connector.AuthSpec{
				Type: "oauth2",
				Config: map[string]any{
					"provider":                "feishu_tenant_access_token",
					"client_id_ref":           "env:FEISHU_APP_ID",
					"client_secret_ref":       "env:FEISHU_APP_SECRET",
					"tenant_access_token_url": server.URL + "/token",
				},
			},
		},
		Operation: connector.OperationSpec{
			ID: "connop_feishu_create_document",
			Request: connector.OperationRequest{
				Method: "POST",
				Path:   "/open-apis/docx/v1/documents",
			},
		},
		Input: map[string]any{"title": "doc"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["success"] != true {
		t.Fatalf("expected success=true, got %#v", result.Output["success"])
	}
}

func TestHTTPProtocolExecutorUsesPlaceholderNameAsDefaultPathParam(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/documents/doc_123/blocks/doc_123/descendant" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0})
	}))
	defer server.Close()

	executor := HTTPProtocolExecutor{Client: server.Client()}
	result, err := executor.Execute(context.Background(), connector.ProtocolRequest{
		Connector: connector.ConnectorSpec{
			ID:       "conn_feishu_doc",
			Protocol: connector.ProtocolSpec{Kind: HTTPProtocolKind, BaseURL: server.URL},
		},
		Operation: connector.OperationSpec{
			ID: "connop_feishu_create_nested_blocks",
			Request: connector.OperationRequest{
				Method: "POST",
				Path:   "/documents/{document_id}/blocks/{block_id}/descendant",
			},
		},
		Input: map[string]any{
			"document_id": "doc_123",
			"block_id":    "doc_123",
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Output["success"] != true {
		t.Fatalf("expected success=true, got %#v", result.Output)
	}
}

func TestEnvSecretResolverSupportsEnvReferenceSyntax(t *testing.T) {
	t.Setenv("FEISHU_APP_ID", "app-id")
	resolver := EnvSecretResolver{}
	for _, ref := range []string{"env:FEISHU_APP_ID", "$FEISHU_APP_ID", "FEISHU_APP_ID"} {
		t.Run(ref, func(t *testing.T) {
			value, ok := resolver.ResolveSecret(ref)
			if !ok || value != "app-id" {
				t.Fatalf("ResolveSecret(%q) = %q, %v", ref, value, ok)
			}
		})
	}
}

type mapSecretResolver map[string]string

func (r mapSecretResolver) ResolveSecret(ref string) (string, bool) {
	value, ok := r[ref]
	if ok {
		return value, true
	}
	return "", false
}
