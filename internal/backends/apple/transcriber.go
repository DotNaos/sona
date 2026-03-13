package apple

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/DotNaos/sona/internal/speech"
)

const (
	providerName  = "apple"
	modelName     = "SpeechTranscriber"
	helperProduct = "sona-apple-stt"
	helperRelPath = "../../apple/speechhelper"
)

type helperOutput struct {
	Text         string `json:"text"`
	Final        bool   `json:"final"`
	FinishReason string `json:"finish_reason"`
	DurationMS   int64  `json:"duration_ms"`
}

type Transcriber struct {
	buildHelper func(ctx context.Context) (string, error)
	runHelper   func(ctx context.Context, helperPath string) ([]byte, error)
}

func New() *Transcriber {
	t := &Transcriber{}
	t.buildHelper = t.defaultBuildHelper
	t.runHelper = func(ctx context.Context, helperPath string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, helperPath)
		cmd.Stderr = os.Stderr
		return cmd.Output()
	}
	return t
}

func (t *Transcriber) Listen(ctx context.Context) (speech.Transcript, error) {
	helperPath, err := t.buildHelper(ctx)
	if err != nil {
		return speech.Transcript{}, err
	}

	output, err := t.runHelper(ctx, helperPath)
	if err != nil {
		return speech.Transcript{}, fmt.Errorf("run apple speech helper: %w", err)
	}

	var parsed helperOutput
	if err := json.Unmarshal(output, &parsed); err != nil {
		return speech.Transcript{}, fmt.Errorf("decode apple speech helper output: %w", err)
	}

	return speech.Transcript{
		Text:         parsed.Text,
		Final:        parsed.Final,
		FinishReason: parsed.FinishReason,
		DurationMS:   parsed.DurationMS,
		Provider:     providerName,
		Model:        modelName,
	}, nil
}

func (t *Transcriber) defaultBuildHelper(ctx context.Context) (string, error) {
	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve apple backend source path")
	}

	packageDir := filepath.Dir(filePath)
	helperDir := filepath.Clean(filepath.Join(packageDir, helperRelPath))

	cmd := exec.CommandContext(ctx, "swift", "build", "-c", "release", "--product", helperProduct)
	cmd.Dir = helperDir
	cmd.Stderr = os.Stderr
	if output, err := cmd.Output(); err != nil {
		return "", fmt.Errorf("build apple speech helper: %w", err)
	} else if len(output) > 0 {
		fmt.Fprint(os.Stderr, string(output))
	}

	return filepath.Join(helperDir, ".build", "release", helperProduct), nil
}
