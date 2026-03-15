package whispercpp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DotNaos/sona/internal/config"
)

func TestLanguageFromLocale(t *testing.T) {
	tests := map[string]string{
		"de-DE":   "de",
		"de_DE":   "de",
		"en-US":   "en",
		" fr-CH ": "fr",
		"":        "",
	}

	for input, want := range tests {
		if got := languageFromLocale(input); got != want {
			t.Fatalf("languageFromLocale(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveModelPathUsesConfiguredModelsDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("SONA_WHISPER_MODEL", "")

	modelDir, err := config.ModelsDir()
	if err != nil {
		t.Fatalf("ModelsDir() error = %v", err)
	}
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	want := filepath.Join(modelDir, "ggml-base.bin")
	if err := os.WriteFile(want, []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	transcriber := New(Options{})
	got, err := transcriber.resolveModelPath()
	if err != nil {
		t.Fatalf("resolveModelPath() error = %v", err)
	}
	if got != want {
		t.Fatalf("resolveModelPath() = %q, want %q", got, want)
	}
}
