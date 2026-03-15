package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/DotNaos/sona/internal/backends/apple"
	"github.com/DotNaos/sona/internal/config"
)

func newDebugCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Run low-level diagnostics for sona audio paths",
	}

	cmd.AddCommand(newDebugRawInputCommand())
	cmd.AddCommand(newDebugSTTLocalesCommand())
	return cmd
}

func newDebugRawInputCommand() *cobra.Command {
	var seconds int

	cmd := &cobra.Command{
		Use:   "raw-input",
		Short: "Capture a short microphone sample without SpeechTranscriber and report audio levels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if err := playListenIndicator(listenIndicatorStart); err != nil {
				return err
			}

			transcriber := apple.New(apple.Options{
				PreferredInputDeviceUID: cfg.PreferredInputDeviceUID,
				LogDeviceSelection:      true,
				DebugListen:             true,
			})
			summary, err := transcriber.RawInputDebug(cmd.Context(), seconds)
			if err != nil {
				return err
			}
			if err := playListenIndicator(listenIndicatorStop); err != nil {
				return err
			}
			return writeJSON(os.Stdout, summary)
		},
	}

	cmd.Flags().IntVar(&seconds, "seconds", 3, "How long to sample the microphone")
	return cmd
}

func newDebugSTTLocalesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stt-locales",
		Short: "List SpeechTranscriber locales and whether the current input format is analyzer-compatible",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			summary, err := apple.New(apple.Options{
				PreferredInputDeviceUID: cfg.PreferredInputDeviceUID,
				LogDeviceSelection:      true,
			}).STTLocales(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSON(os.Stdout, summary)
		},
	}

	return cmd
}
