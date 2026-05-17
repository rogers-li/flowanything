package config

import (
	"testing"
	"time"
)

func TestStringReturnsFallbackWhenUnset(t *testing.T) {
	t.Setenv("FLOW_ANYTHING_TEST_STRING", "")

	if got := String("FLOW_ANYTHING_TEST_STRING", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestDurationParsesGoDurationAndMilliseconds(t *testing.T) {
	t.Setenv("FLOW_ANYTHING_TEST_DURATION", "2s")
	if got := Duration("FLOW_ANYTHING_TEST_DURATION", time.Second); got != 2*time.Second {
		t.Fatalf("expected 2s, got %s", got)
	}

	t.Setenv("FLOW_ANYTHING_TEST_DURATION", "1500")
	if got := Duration("FLOW_ANYTHING_TEST_DURATION", time.Second); got != 1500*time.Millisecond {
		t.Fatalf("expected 1500ms, got %s", got)
	}
}

func TestIntAndBoolFallbackOnInvalidValues(t *testing.T) {
	t.Setenv("FLOW_ANYTHING_TEST_INT", "not-an-int")
	if got := Int("FLOW_ANYTHING_TEST_INT", 7); got != 7 {
		t.Fatalf("expected fallback int, got %d", got)
	}

	t.Setenv("FLOW_ANYTHING_TEST_BOOL", "not-a-bool")
	if got := Bool("FLOW_ANYTHING_TEST_BOOL", true); !got {
		t.Fatal("expected fallback bool true")
	}
}
