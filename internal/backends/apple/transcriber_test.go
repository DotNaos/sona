package apple

import (
	"context"
	"slices"
	"testing"
)

func TestListenWrapsHelperOutput(t *testing.T) {
	transcriber := &Transcriber{
		buildHelper: func(context.Context) (string, error) {
			return "/tmp/helper", nil
		},
		runHelper: func(context.Context, string, ...string) ([]byte, error) {
			return []byte(`{"text":"hello","final":true,"finish_reason":"vad_silence","duration_ms":321}`), nil
		},
	}

	result, err := transcriber.Listen(context.Background())
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	if result.Provider != providerName {
		t.Fatalf("Provider = %q, want %q", result.Provider, providerName)
	}
	if result.Model != modelName {
		t.Fatalf("Model = %q, want %q", result.Model, modelName)
	}
	if result.Text != "hello" {
		t.Fatalf("Text = %q, want hello", result.Text)
	}
}

func TestListInputDevicesWrapsHelperOutput(t *testing.T) {
	transcriber := &Transcriber{
		buildHelper: func(context.Context) (string, error) {
			return "/tmp/helper", nil
		},
		runHelper: func(context.Context, string, ...string) ([]byte, error) {
			return []byte(`[{"name":"Built-in Microphone","uid":"builtin","is_default":true}]`), nil
		},
	}

	devices, err := transcriber.ListInputDevices(context.Background())
	if err != nil {
		t.Fatalf("ListInputDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("len(devices) = %d, want 1", len(devices))
	}
	if devices[0].UID != "builtin" {
		t.Fatalf("UID = %q, want builtin", devices[0].UID)
	}
}

func TestListenPassesLocaleFlagToHelper(t *testing.T) {
	transcriber := &Transcriber{
		options: Options{Locale: "de-DE"},
		buildHelper: func(context.Context) (string, error) {
			return "/tmp/helper", nil
		},
		runHelper: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			if !slices.Contains(args, "--locale") {
				t.Fatalf("args %v missing --locale", args)
			}
			if !slices.Contains(args, "de-DE") {
				t.Fatalf("args %v missing locale value", args)
			}
			return []byte(`{"text":"hallo","final":true,"finish_reason":"vad_silence","duration_ms":321}`), nil
		},
	}

	if _, err := transcriber.Listen(context.Background()); err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
}
