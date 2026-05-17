package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	apperrors "flow-anything/internal/platform/kernel/errors"
)

func TestClientGetJSONDecodesApplicationError(t *testing.T) {
	t.Parallel()

	client := NewWithHTTPClient("http://service.test", &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"not_found","message":"missing"}}`)),
			}, nil
		}),
	})

	var out map[string]any
	err := client.GetJSON(context.Background(), "/missing", nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}

	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected app error, got %T", err)
	}
	if appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected code %q, got %q", apperrors.CodeNotFound, appErr.Code)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
