package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DotNaos/sona/internal/backends/apple"
	"github.com/DotNaos/sona/internal/config"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect or update sona configuration",
	}

	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigInputCommand())
	cmd.AddCommand(newConfigWhisperCommand())
	return cmd
}

func newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the current sona configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			path, err := config.Path()
			if err != nil {
				return err
			}

			payload := struct {
				Path   string        `json:"path"`
				Config config.Config `json:"config"`
			}{
				Path:   path,
				Config: cfg,
			}
			return writeJSON(os.Stdout, payload)
		},
	}
}

func newConfigInputCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Manage microphone device selection for sona listen",
	}

	cmd.AddCommand(newConfigInputListCommand())
	cmd.AddCommand(newConfigInputCurrentCommand())
	cmd.AddCommand(newConfigInputSetCommand())
	cmd.AddCommand(newConfigInputClearCommand())
	return cmd
}

func newConfigWhisperCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whisper",
		Short: "Inspect or update whisper.cpp settings used by sona listen",
	}

	cmd.AddCommand(newConfigWhisperShowCommand())
	cmd.AddCommand(newConfigWhisperModelSetCommand())
	cmd.AddCommand(newConfigWhisperModelClearCommand())
	cmd.AddCommand(newConfigWhisperCLISetCommand())
	cmd.AddCommand(newConfigWhisperCLIClearCommand())
	return cmd
}

func newConfigWhisperShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show the current whisper.cpp configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return writeJSON(os.Stdout, map[string]string{
				"whisper_cli_path":   cfg.WhisperCLIPath,
				"whisper_model_path": cfg.WhisperModelPath,
			})
		},
	}
}

func newConfigWhisperModelSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "model set <path>",
		Short: "Persist the default whisper.cpp model path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := strings.TrimSpace(args[0])
			if path == "" {
				return fmt.Errorf("model path must not be empty")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.WhisperModelPath = path
			if err := config.Save(cfg); err != nil {
				return err
			}
			return writeJSON(os.Stdout, map[string]string{"whisper_model_path": path})
		},
	}
}

func newConfigWhisperModelClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "model clear",
		Short: "Clear the persisted whisper.cpp model path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.WhisperModelPath = ""
			if err := config.Save(cfg); err != nil {
				return err
			}
			return writeJSON(os.Stdout, map[string]string{"whisper_model_path": ""})
		},
	}
}

func newConfigWhisperCLISetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cli set <path>",
		Short: "Persist the default whisper-cli path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := strings.TrimSpace(args[0])
			if path == "" {
				return fmt.Errorf("whisper-cli path must not be empty")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.WhisperCLIPath = path
			if err := config.Save(cfg); err != nil {
				return err
			}
			return writeJSON(os.Stdout, map[string]string{"whisper_cli_path": path})
		},
	}
}

func newConfigWhisperCLIClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cli clear",
		Short: "Clear the persisted whisper-cli path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.WhisperCLIPath = ""
			if err := config.Save(cfg); err != nil {
				return err
			}
			return writeJSON(os.Stdout, map[string]string{"whisper_cli_path": ""})
		},
	}
}

func newConfigInputListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available input devices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			devices, err := apple.New(apple.Options{}).ListInputDevices(cmd.Context())
			if err != nil {
				return err
			}

			type listedDevice struct {
				Name        string `json:"name"`
				UID         string `json:"uid"`
				IsDefault   bool   `json:"is_default"`
				IsPreferred bool   `json:"is_preferred"`
			}

			result := make([]listedDevice, 0, len(devices))
			for _, device := range devices {
				result = append(result, listedDevice{
					Name:        device.Name,
					UID:         device.UID,
					IsDefault:   device.IsDefault,
					IsPreferred: device.UID != "" && device.UID == cfg.PreferredInputDeviceUID,
				})
			}

			return writeJSON(os.Stdout, result)
		},
	}
}

func newConfigInputCurrentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Show the current macOS default input device",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			current, err := apple.New(apple.Options{}).CurrentInputDevice(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSON(os.Stdout, current)
		},
	}
}

func newConfigInputSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <uid-or-name>",
		Short: "Persist the preferred microphone for sona listen",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			devices, err := apple.New(apple.Options{}).ListInputDevices(cmd.Context())
			if err != nil {
				return err
			}

			device, err := resolveInputDevice(context.Background(), devices, args[0])
			if err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.PreferredInputDeviceUID = device.UID
			if err := config.Save(cfg); err != nil {
				return err
			}

			return writeJSON(os.Stdout, struct {
				SavedPreferredInputDeviceUID string            `json:"saved_preferred_input_device_uid"`
				Device                       apple.InputDevice `json:"device"`
			}{
				SavedPreferredInputDeviceUID: device.UID,
				Device:                       device,
			})
		},
	}
}

func newConfigInputClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the preferred microphone and fall back to the system default",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.PreferredInputDeviceUID = ""
			if err := config.Save(cfg); err != nil {
				return err
			}
			return writeJSON(os.Stdout, map[string]string{
				"saved_preferred_input_device_uid": "",
			})
		},
	}
}

func resolveInputDevice(_ context.Context, devices []apple.InputDevice, query string) (apple.InputDevice, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return apple.InputDevice{}, fmt.Errorf("device query is empty")
	}

	for _, device := range devices {
		if device.UID == query {
			return device, nil
		}
	}

	lowerQuery := strings.ToLower(query)
	exactNameMatches := make([]apple.InputDevice, 0, 1)
	partialMatches := make([]apple.InputDevice, 0, 4)

	for _, device := range devices {
		lowerName := strings.ToLower(device.Name)
		switch {
		case lowerName == lowerQuery:
			exactNameMatches = append(exactNameMatches, device)
		case strings.Contains(lowerName, lowerQuery):
			partialMatches = append(partialMatches, device)
		}
	}

	switch {
	case len(exactNameMatches) == 1:
		return exactNameMatches[0], nil
	case len(exactNameMatches) > 1:
		return apple.InputDevice{}, fmt.Errorf("device name %q is ambiguous", query)
	case len(partialMatches) == 1:
		return partialMatches[0], nil
	case len(partialMatches) > 1:
		return apple.InputDevice{}, fmt.Errorf("device query %q matches multiple inputs", query)
	default:
		return apple.InputDevice{}, fmt.Errorf("no input device matches %q", query)
	}
}
