package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DotNaos/sona/internal/backends/apple"
	"github.com/DotNaos/sona/internal/backends/gemini"
	"github.com/DotNaos/sona/internal/backends/whispercpp"
	"github.com/DotNaos/sona/internal/config"
	"github.com/DotNaos/sona/internal/queue"
	"github.com/DotNaos/sona/internal/speech"
)

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "sona",
		Short:         "Provider-neutral speech CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(newSayCommand())
	rootCmd.AddCommand(newListenCommand())
	rootCmd.AddCommand(newConfigCommand())
	rootCmd.AddCommand(newDebugCommand())
	rootCmd.AddCommand(newSetupCommand())

	return rootCmd
}

func newSayCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "say [text]",
		Short: "Speak text through the current speaker",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := resolveTextInput(args, os.Stdin)
			if err != nil {
				return err
			}

			sayQueue, err := queue.NewSayQueue()
			if err != nil {
				return err
			}

			slot, err := sayQueue.Acquire(cmd.Context())
			if err != nil {
				return err
			}
			defer slot.Release()

			return speakText(cmd, text)
		},
	}

	return cmd
}

func newListenCommand() *cobra.Command {
	var prompt string
	var debugListen bool
	var locale string
	var backend string
	var whisperModel string
	var whisperCLI string

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Listen through the current microphone and print final JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			backend = strings.ToLower(strings.TrimSpace(backend))
			locale = strings.TrimSpace(locale)
			if locale == "" {
				return fmt.Errorf("--locale must not be empty")
			}
			if whisperModel == "" {
				whisperModel = strings.TrimSpace(cfg.WhisperModelPath)
			}
			if whisperCLI == "" {
				whisperCLI = strings.TrimSpace(cfg.WhisperCLIPath)
			}

			prompt = strings.TrimSpace(prompt)
			if prompt != "" {
				sayQueue, err := queue.NewSayQueue()
				if err != nil {
					return err
				}

				slot, err := sayQueue.Acquire(cmd.Context())
				if err != nil {
					return err
				}
				defer slot.Release()

				if err := speakText(cmd, prompt); err != nil {
					return err
				}
			}

			if err := playListenIndicator(listenIndicatorStart); err != nil {
				return err
			}

			var transcript speech.Transcript
			switch backend {
			case "apple":
				transcript, err = apple.New(apple.Options{
					PreferredInputDeviceUID: cfg.PreferredInputDeviceUID,
					LogDeviceSelection:      true,
					DebugListen:             debugListen,
					Locale:                  locale,
				}).Listen(cmd.Context())
			case "", "whispercpp":
				transcript, err = whispercpp.New(whispercpp.Options{
					PreferredInputDeviceUID: cfg.PreferredInputDeviceUID,
					LogDeviceSelection:      true,
					DebugListen:             debugListen,
					Locale:                  locale,
					ModelPath:               whisperModel,
					BinaryPath:              whisperCLI,
				}).Listen(cmd.Context())
			default:
				return fmt.Errorf("unsupported STT backend %q; use apple or whispercpp", backend)
			}
			if err != nil {
				return err
			}
			if err := playListenIndicator(listenIndicatorStop); err != nil {
				return err
			}
			return writeJSON(os.Stdout, transcript)
		},
	}

	cmd.Flags().StringVar(&prompt, "prompt", "", "Speak this prompt immediately before listening")
	cmd.Flags().StringVar(&locale, "locale", "", "Required STT locale, for example de-DE or en-US")
	cmd.Flags().StringVar(&backend, "backend", "whispercpp", "STT backend to use: whispercpp (default) or apple")
	cmd.Flags().StringVar(&whisperModel, "whisper-model", "", "Path to a local whisper.cpp model file; defaults to the configured or auto-detected model")
	cmd.Flags().StringVar(&whisperCLI, "whisper-cli", "", "Path to whisper-cli, if it is not available in PATH")
	cmd.Flags().BoolVar(&debugListen, "debug-listen", false, "Log microphone levels and speech transcriber events to stderr")
	_ = cmd.MarkFlagRequired("locale")
	return cmd
}

func speakText(cmd *cobra.Command, text string) error {
	apiKey := gemini.ResolveAPIKey()
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY is required for speech output")
	}

	synthesizer := gemini.New(apiKey)
	return synthesizer.Synthesize(cmd.Context(), text, &lazyPCMPlayer{})
}
