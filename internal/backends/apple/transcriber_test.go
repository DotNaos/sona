package apple

import (
	"context"
	"testing"
)

func TestListenWrapsHelperOutput(t *testing.T) {
	transcriber := &Transcriber{
		buildHelper: func(context.Context) (string, error) {
			return "/tmp/helper", nil
		},
		runHelper: func(context.Context, string) ([]byte, error) {
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
