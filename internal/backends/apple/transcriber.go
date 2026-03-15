package apple

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

type InputDevice struct {
	Name      string `json:"name"`
	UID       string `json:"uid"`
	IsDefault bool   `json:"is_default"`
}

type RawInputDebugSummary struct {
	InputDevice      InputDevice `json:"input_device"`
	DurationMS       int64       `json:"duration_ms"`
	Buffers          int         `json:"buffers"`
	NonSilentBuffers int         `json:"non_silent_buffers"`
	AverageLevel     float64     `json:"average_level"`
	MaxLevel         float64     `json:"max_level"`
}

type CaptureUtteranceSummary struct {
	AudioPath    string      `json:"audio_path"`
	DurationMS   int64       `json:"duration_ms"`
	FinishReason string      `json:"finish_reason"`
	InputDevice  InputDevice `json:"input_device"`
}

type STTLocaleSupport struct {
	Identifier         string `json:"identifier"`
	AnalyzerCompatible bool   `json:"analyzer_compatible"`
}

type STTLocalesSummary struct {
	InputDevice     InputDevice        `json:"input_device"`
	InputSampleRate float64            `json:"input_sample_rate"`
	InputChannels   uint32             `json:"input_channels"`
	Locales         []STTLocaleSupport `json:"locales"`
}

type Options struct {
	PreferredInputDeviceUID string
	LogDeviceSelection      bool
	DebugListen             bool
	Locale                  string
}

type Transcriber struct {
	options     Options
	buildHelper func(ctx context.Context) (string, error)
	runHelper   func(ctx context.Context, helperPath string, args ...string) ([]byte, error)
}

func New(options Options) *Transcriber {
	t := &Transcriber{options: options}
	t.buildHelper = t.defaultBuildHelper
	t.runHelper = func(ctx context.Context, helperPath string, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, helperPath, args...)
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

	args := []string{"listen"}
	if uid := strings.TrimSpace(t.options.PreferredInputDeviceUID); uid != "" {
		args = append(args, "--device-uid", uid)
	}
	if locale := strings.TrimSpace(t.options.Locale); locale != "" {
		args = append(args, "--locale", locale)
	}
	if t.options.LogDeviceSelection {
		args = append(args, "--log-device")
	}
	if t.options.DebugListen {
		args = append(args, "--debug")
	}

	output, err := t.runHelper(ctx, helperPath, args...)
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

func (t *Transcriber) ListInputDevices(ctx context.Context) ([]InputDevice, error) {
	helperPath, err := t.buildHelper(ctx)
	if err != nil {
		return nil, err
	}

	output, err := t.runHelper(ctx, helperPath, "list-inputs")
	if err != nil {
		return nil, fmt.Errorf("list input devices: %w", err)
	}

	var devices []InputDevice
	if err := json.Unmarshal(output, &devices); err != nil {
		return nil, fmt.Errorf("decode input devices: %w", err)
	}
	return devices, nil
}

func (t *Transcriber) CurrentInputDevice(ctx context.Context) (InputDevice, error) {
	helperPath, err := t.buildHelper(ctx)
	if err != nil {
		return InputDevice{}, err
	}

	output, err := t.runHelper(ctx, helperPath, "current-input")
	if err != nil {
		return InputDevice{}, fmt.Errorf("get current input device: %w", err)
	}

	var device InputDevice
	if err := json.Unmarshal(output, &device); err != nil {
		return InputDevice{}, fmt.Errorf("decode current input device: %w", err)
	}
	return device, nil
}

func (t *Transcriber) RawInputDebug(ctx context.Context, seconds int) (RawInputDebugSummary, error) {
	helperPath, err := t.buildHelper(ctx)
	if err != nil {
		return RawInputDebugSummary{}, err
	}

	args := []string{"raw-input", "--seconds", fmt.Sprintf("%d", seconds)}
	if uid := strings.TrimSpace(t.options.PreferredInputDeviceUID); uid != "" {
		args = append(args, "--device-uid", uid)
	}
	if t.options.LogDeviceSelection {
		args = append(args, "--log-device")
	}
	if t.options.DebugListen {
		args = append(args, "--debug")
	}

	output, err := t.runHelper(ctx, helperPath, args...)
	if err != nil {
		return RawInputDebugSummary{}, fmt.Errorf("run raw input debug: %w", err)
	}

	var summary RawInputDebugSummary
	if err := json.Unmarshal(output, &summary); err != nil {
		return RawInputDebugSummary{}, fmt.Errorf("decode raw input debug output: %w", err)
	}
	return summary, nil
}

func (t *Transcriber) CaptureUtterance(ctx context.Context) (CaptureUtteranceSummary, error) {
	helperPath, err := t.buildHelper(ctx)
	if err != nil {
		return CaptureUtteranceSummary{}, err
	}

	args := []string{"capture-utterance"}
	if uid := strings.TrimSpace(t.options.PreferredInputDeviceUID); uid != "" {
		args = append(args, "--device-uid", uid)
	}
	if t.options.LogDeviceSelection {
		args = append(args, "--log-device")
	}
	if t.options.DebugListen {
		args = append(args, "--debug")
	}

	output, err := t.runHelper(ctx, helperPath, args...)
	if err != nil {
		return CaptureUtteranceSummary{}, fmt.Errorf("run utterance capture: %w", err)
	}

	var summary CaptureUtteranceSummary
	if err := json.Unmarshal(output, &summary); err != nil {
		return CaptureUtteranceSummary{}, fmt.Errorf("decode utterance capture output: %w", err)
	}
	return summary, nil
}

func (t *Transcriber) STTLocales(ctx context.Context) (STTLocalesSummary, error) {
	helperPath, err := t.buildHelper(ctx)
	if err != nil {
		return STTLocalesSummary{}, err
	}

	args := []string{"stt-locales"}
	if uid := strings.TrimSpace(t.options.PreferredInputDeviceUID); uid != "" {
		args = append(args, "--device-uid", uid)
	}
	if t.options.LogDeviceSelection {
		args = append(args, "--log-device")
	}

	output, err := t.runHelper(ctx, helperPath, args...)
	if err != nil {
		return STTLocalesSummary{}, fmt.Errorf("list STT locales: %w", err)
	}

	var summary STTLocalesSummary
	if err := json.Unmarshal(output, &summary); err != nil {
		return STTLocalesSummary{}, fmt.Errorf("decode STT locale output: %w", err)
	}
	return summary, nil
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
