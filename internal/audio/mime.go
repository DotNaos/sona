package audio

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/DotNaos/sona/internal/speech"
)

func ParsePCMFormat(mimeType string) (speech.PCMFormat, error) {
	parts := strings.Split(mimeType, ";")
	if len(parts) == 0 {
		return speech.PCMFormat{}, fmt.Errorf("missing mime type")
	}

	fileType := strings.TrimSpace(parts[0])
	segments := strings.Split(fileType, "/")
	if len(segments) != 2 {
		return speech.PCMFormat{}, fmt.Errorf("invalid mime type %q", mimeType)
	}

	format := speech.PCMFormat{
		Channels:      1,
		BitsPerSample: 16,
	}

	subtype := strings.ToLower(strings.TrimSpace(segments[1]))
	switch {
	case subtype == "pcm":
	case strings.HasPrefix(subtype, "l"):
		bits, err := strconv.Atoi(strings.TrimPrefix(subtype, "l"))
		if err != nil {
			return speech.PCMFormat{}, fmt.Errorf("invalid pcm subtype %q", subtype)
		}
		format.BitsPerSample = uint16(bits)
	default:
		return speech.PCMFormat{}, fmt.Errorf("unsupported audio subtype %q", subtype)
	}

	for _, rawParam := range parts[1:] {
		param := strings.TrimSpace(rawParam)
		if param == "" {
			continue
		}

		keyValue := strings.SplitN(param, "=", 2)
		if len(keyValue) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(keyValue[0]))
		value := strings.Trim(strings.TrimSpace(keyValue[1]), "\"")

		switch key {
		case "rate":
			rate, err := strconv.Atoi(value)
			if err != nil {
				return speech.PCMFormat{}, fmt.Errorf("invalid sample rate %q", value)
			}
			format.SampleRate = uint32(rate)
		case "channels":
			channels, err := strconv.Atoi(value)
			if err != nil {
				return speech.PCMFormat{}, fmt.Errorf("invalid channel count %q", value)
			}
			format.Channels = uint32(channels)
		}
	}

	if format.SampleRate == 0 {
		return speech.PCMFormat{}, fmt.Errorf("missing sample rate in mime type %q", mimeType)
	}
	if format.BitsPerSample != 16 {
		return speech.PCMFormat{}, fmt.Errorf("unsupported bits per sample %d", format.BitsPerSample)
	}
	if format.Channels == 0 {
		return speech.PCMFormat{}, fmt.Errorf("missing channel count in mime type %q", mimeType)
	}

	return format, nil
}
