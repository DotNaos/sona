package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var stdinIsInteractive = func() (bool, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false, err
	}
	return stat.Mode()&os.ModeCharDevice != 0, nil
}

func resolveTextInput(args []string, stdin io.Reader) (string, error) {
	if len(args) > 0 {
		text := strings.TrimSpace(strings.Join(args, " "))
		if text == "" {
			return "", fmt.Errorf("text input is empty")
		}
		return text, nil
	}

	isInteractive, err := stdinIsInteractive()
	if err != nil {
		return "", fmt.Errorf("stat stdin: %w", err)
	}
	if isInteractive {
		return "", fmt.Errorf("missing text input")
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		return "", fmt.Errorf("text input is empty")
	}

	return text, nil
}
