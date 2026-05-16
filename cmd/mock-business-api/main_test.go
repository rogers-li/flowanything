package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"flow-anything/internal/mockbusiness"
)

func TestWeatherCurrentRequiresCity(t *testing.T) {
	t.Parallel()

	server := newMockBusinessTestServer()
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/weather/current", nil)

	server.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.Code)
	}
}

func TestWeatherCurrentReturnsDeterministicWeather(t *testing.T) {
	t.Parallel()

	server := newMockBusinessTestServer()
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/weather/current?city=深圳", nil)

	server.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["city"] != "深圳" {
		t.Fatalf("unexpected city %#v", body["city"])
	}
	if body["condition"] != "多云" {
		t.Fatalf("unexpected condition %#v", body["condition"])
	}
	if body["source"] != "mock-business-api" {
		t.Fatalf("unexpected source %#v", body["source"])
	}
}

func newMockBusinessTestServer() *http.ServeMux {
	mux := http.NewServeMux()
	mockbusiness.RegisterRoutes(mux)
	return mux
}
