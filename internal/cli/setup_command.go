package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/DotNaos/sona/internal/config"
)

func newSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install or prepare local dependencies for sona",
	}

	cmd.AddCommand(newSetupWhisperCPPCommand())
	return cmd
}

func newSetupWhisperCPPCommand() *cobra.Command {
	var model string
	var force bool

	cmd := &cobra.Command{
		Use:   "whispercpp",
		Short: "Install whisper.cpp and download a local model into the sona config directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			model = strings.TrimSpace(strings.ToLower(model))
			modelURL, fileName, err := whisperModelDownload(model)
			if err != nil {
				return err
			}

			cliPath, err := ensureWhisperCLI(cmd)
			if err != nil {
				return err
			}

			modelsDir, err := config.ModelsDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(modelsDir, 0o755); err != nil {
				return fmt.Errorf("create models dir: %w", err)
			}

			modelPath := filepath.Join(modelsDir, fileName)
			if _, err := os.Stat(modelPath); err == nil && !force {
				// keep existing file
			} else {
				if err := downloadFile(modelURL, modelPath); err != nil {
					return err
				}
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.WhisperCLIPath = cliPath
			cfg.WhisperModelPath = modelPath
			if err := config.Save(cfg); err != nil {
				return err
			}

			return writeJSON(os.Stdout, map[string]string{
				"backend":            "whispercpp",
				"whisper_cli_path":   cliPath,
				"whisper_model_path": modelPath,
				"model":              model,
				"next_command":       "sona listen --locale de-DE",
			})
		},
	}

	cmd.Flags().StringVar(&model, "model", "base", "whisper.cpp model to download: tiny, base, small")
	cmd.Flags().BoolVar(&force, "force", false, "Re-download the selected model even if it already exists")
	return cmd
}

func ensureWhisperCLI(cmd *cobra.Command) (string, error) {
	if path, err := exec.LookPath("whisper-cli"); err == nil {
		return path, nil
	}

	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return "", fmt.Errorf("whisper-cli was not found and Homebrew is unavailable; install whisper.cpp manually or set `whisper_cli_path` in `sona config`")
	}

	installCmd := exec.CommandContext(cmd.Context(), brewPath, "install", "whisper-cpp")
	installCmd.Stdout = os.Stderr
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return "", fmt.Errorf("install whisper-cpp with Homebrew: %w", err)
	}

	path, err := exec.LookPath("whisper-cli")
	if err != nil {
		return "", fmt.Errorf("whisper-cpp installed, but whisper-cli is still not in PATH")
	}
	return path, nil
}

func whisperModelDownload(model string) (url string, fileName string, err error) {
	switch model {
	case "tiny":
		return "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin", "ggml-tiny.bin", nil
	case "base":
		return "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin", "ggml-base.bin", nil
	case "small":
		return "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin", "ggml-small.bin", nil
	default:
		return "", "", fmt.Errorf("unsupported whisper.cpp model %q; use tiny, base, or small", model)
	}
}

func downloadFile(url, destination string) error {
	tmpPath := destination + ".tmp"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download whisper model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download whisper model: unexpected HTTP status %s", resp.Status)
	}

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create model file: %w", err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write model file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close model file: %w", err)
	}

	if err := os.Rename(tmpPath, destination); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("finalize model file: %w", err)
	}

	// brief settle to avoid immediate follow-up stat races on slower filesystems
	time.Sleep(100 * time.Millisecond)
	return nil
}
