package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	cfg := Config{
		PreferredInputDeviceUID: "device-123",
		WhisperModelPath:        "/tmp/ggml-base.bin",
		WhisperCLIPath:          "/opt/homebrew/bin/whisper-cli",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.PreferredInputDeviceUID != cfg.PreferredInputDeviceUID {
		t.Fatalf("PreferredInputDeviceUID = %q, want %q", got.PreferredInputDeviceUID, cfg.PreferredInputDeviceUID)
	}
	if got.WhisperModelPath != cfg.WhisperModelPath {
		t.Fatalf("WhisperModelPath = %q, want %q", got.WhisperModelPath, cfg.WhisperModelPath)
	}
	if got.WhisperCLIPath != cfg.WhisperCLIPath {
		t.Fatalf("WhisperCLIPath = %q, want %q", got.WhisperCLIPath, cfg.WhisperCLIPath)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file missing: %v", err)
	}
	if filepath.Base(path) != fileName {
		t.Fatalf("config path = %q, want basename %q", path, fileName)
	}

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if filepath.Base(dir) != "sona" {
		t.Fatalf("config dir = %q, want basename sona", dir)
	}

	modelsDir, err := ModelsDir()
	if err != nil {
		t.Fatalf("ModelsDir() error = %v", err)
	}
	if filepath.Base(modelsDir) != "models" {
		t.Fatalf("models dir = %q, want basename models", modelsDir)
	}
}
