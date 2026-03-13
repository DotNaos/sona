package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/DotNaos/sona/internal/backends/apple"
	"github.com/DotNaos/sona/internal/backends/gemini"
	"github.com/DotNaos/sona/internal/queue"
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

			apiKey := gemini.ResolveAPIKey()
			if apiKey == "" {
				return fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY is required for `sona say`")
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

			synthesizer := gemini.New(apiKey)
			return synthesizer.Synthesize(cmd.Context(), text, &lazyPCMPlayer{})
		},
	}

	return cmd
}

func newListenCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "listen",
		Short: "Listen through the current microphone and print final JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			transcriber := apple.New()
			transcript, err := transcriber.Listen(cmd.Context())
			if err != nil {
				return err
			}
			return writeJSON(os.Stdout, transcript)
		},
	}
}
