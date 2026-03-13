package cli

import (
	"strings"
	"testing"
)

func withMockInteractiveStdin(t *testing.T, interactive bool) {
	t.Helper()
	previous := stdinIsInteractive
	stdinIsInteractive = func() (bool, error) {
		return interactive, nil
	}
	t.Cleanup(func() {
		stdinIsInteractive = previous
	})
}

func TestResolveTextInputPrefersArgs(t *testing.T) {
	withMockInteractiveStdin(t, true)

	text, err := resolveTextInput([]string{"hello", "world"}, strings.NewReader("ignored"))
	if err != nil {
		t.Fatalf("resolveTextInput() error = %v", err)
	}
	if text != "hello world" {
		t.Fatalf("resolveTextInput() = %q, want %q", text, "hello world")
	}
}

func TestResolveTextInputReadsStdin(t *testing.T) {
	withMockInteractiveStdin(t, false)

	text, err := resolveTextInput(nil, strings.NewReader("hello from stdin"))
	if err != nil {
		t.Fatalf("resolveTextInput() error = %v", err)
	}
	if text != "hello from stdin" {
		t.Fatalf("resolveTextInput() = %q, want %q", text, "hello from stdin")
	}
}
