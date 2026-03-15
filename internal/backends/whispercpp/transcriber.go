package whispercpp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DotNaos/sona/internal/backends/apple"
	"github.com/DotNaos/sona/internal/config"
	"github.com/DotNaos/sona/internal/speech"
)

const (
	providerName = "whispercpp"
	modelName    = "whisper-cli"
)

type Options struct {
	PreferredInputDeviceUID string
	LogDeviceSelection      bool
	DebugListen             bool
	Locale                  string
	ModelPath               string
	BinaryPath              string
}

type Transcriber struct {
	options       Options
	appleBackend  *apple.Transcriber
	lookPath      func(file string) (string, error)
	commandRunner func(ctx context.Context, name string, args ...string) error
	readFile      func(path string) ([]byte, error)
	removeFile    func(path string) error
	mkdirTemp     func(dir, pattern string) (string, error)
}

func New(options Options) *Transcriber {
	return &Transcriber{
		options: options,
		appleBackend: apple.New(apple.Options{
			PreferredInputDeviceUID: options.PreferredInputDeviceUID,
			LogDeviceSelection:      options.LogDeviceSelection,
			DebugListen:             options.DebugListen,
		}),
		lookPath: exec.LookPath,
		commandRunner: func(ctx context.Context, name string, args ...string) error {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			return cmd.Run()
		},
		readFile:   os.ReadFile,
		removeFile: os.Remove,
		mkdirTemp:  os.MkdirTemp,
	}
}

func (t *Transcriber) Listen(ctx context.Context) (speech.Transcript, error) {
	modelPath, err := t.resolveModelPath()
	if err != nil {
		return speech.Transcript{}, err
	}

	binaryPath, err := t.resolveBinaryPath()
	if err != nil {
		return speech.Transcript{}, err
	}

	capture, err := t.appleBackend.CaptureUtterance(ctx)
	if err != nil {
		return speech.Transcript{}, err
	}
	if strings.TrimSpace(capture.AudioPath) == "" {
		return speech.Transcript{
			Text:         "",
			Final:        true,
			FinishReason: capture.FinishReason,
			DurationMS:   capture.DurationMS,
			Provider:     providerName,
			Model:        filepath.Base(modelPath),
		}, nil
	}
	defer func() { _ = t.removeFile(capture.AudioPath) }()

	outputDir, err := t.mkdirTemp("", "sona-whispercpp-*")
	if err != nil {
		return speech.Transcript{}, fmt.Errorf("create whisper output dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(outputDir) }()

	outputPrefix := filepath.Join(outputDir, "transcript")
	args := []string{
		"-m", modelPath,
		"-f", capture.AudioPath,
		"-of", outputPrefix,
		"-otxt",
		"-nt",
		"-np",
	}
	if language := languageFromLocale(t.options.Locale); language != "" {
		args = append(args, "-l", language)
	}

	if err := t.commandRunner(ctx, binaryPath, args...); err != nil {
		return speech.Transcript{}, fmt.Errorf("run whisper.cpp: %w", err)
	}

	textPath := outputPrefix + ".txt"
	textBytes, err := t.readFile(textPath)
	if err != nil {
		return speech.Transcript{}, fmt.Errorf("read whisper.cpp transcript: %w", err)
	}

	return speech.Transcript{
		Text:         strings.TrimSpace(string(textBytes)),
		Final:        true,
		FinishReason: capture.FinishReason,
		DurationMS:   capture.DurationMS,
		Provider:     providerName,
		Model:        filepath.Base(modelPath),
	}, nil
}

func (t *Transcriber) resolveBinaryPath() (string, error) {
	if path := strings.TrimSpace(t.options.BinaryPath); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("SONA_WHISPER_CLI")); path != "" {
		return path, nil
	}

	for _, candidate := range []string{"whisper-cli", "/opt/homebrew/bin/whisper-cli", "/usr/local/bin/whisper-cli"} {
		if resolved, err := t.lookPath(candidate); err == nil {
			return resolved, nil
		}
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("whisper-cli was not found; run `sona setup whispercpp`, install whisper.cpp manually, or pass --whisper-cli /path/to/whisper-cli")
}

func languageFromLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return ""
	}
	replacer := strings.NewReplacer("-", "_")
	locale = replacer.Replace(locale)
	parts := strings.Split(locale, "_")
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[0]))
}

func (t *Transcriber) resolveModelPath() (string, error) {
	if path := strings.TrimSpace(t.options.ModelPath); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("SONA_WHISPER_MODEL")); path != "" {
		return path, nil
	}

	modelsDir, err := config.ModelsDir()
	if err == nil {
		preferred := filepath.Join(modelsDir, "ggml-base.bin")
		if _, statErr := os.Stat(preferred); statErr == nil {
			return preferred, nil
		}

		matches, globErr := filepath.Glob(filepath.Join(modelsDir, "ggml-*.bin"))
		if globErr == nil && len(matches) > 0 {
			sort.Strings(matches)
			return matches[0], nil
		}
	}

	return "", fmt.Errorf("no whisper.cpp model was configured or auto-detected; run `sona setup whispercpp`, set SONA_WHISPER_MODEL, or pass --whisper-model /path/to/model.bin")
}
