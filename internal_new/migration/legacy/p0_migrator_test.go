package legacy

import "testing"

func TestPutAuthLocationConvertsQueryHeaderName(t *testing.T) {
	config := map[string]any{}
	putAuthLocation(config, "query:key")

	if config["in"] != "query" {
		t.Fatalf("expected query auth location, got %#v", config["in"])
	}
	if config["name"] != "key" {
		t.Fatalf("expected api key query name, got %#v", config["name"])
	}
}

func TestPutAuthLocationConvertsHeaderName(t *testing.T) {
	config := map[string]any{}
	putAuthLocation(config, "X-Api-Key")

	if config["in"] != "header" {
		t.Fatalf("expected header auth location, got %#v", config["in"])
	}
	if config["name"] != "X-Api-Key" {
		t.Fatalf("expected api key header name, got %#v", config["name"])
	}
}
