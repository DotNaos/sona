package gemini

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"

	"github.com/DotNaos/sona/internal/audio"
	"github.com/DotNaos/sona/internal/speech"
)

const (
	DefaultModel         = "models/gemini-2.5-flash-native-audio-preview-12-2025"
	DefaultVoice         = "Zephyr"
	verbatimSystemPrompt = "Read the user's text exactly as written. Do not add words. Do not remove words. Do not summarize. Do not explain. Do not answer. Do not translate. Do not normalize punctuation, casing, numbers, or symbols. Output only spoken audio for the exact provided text."
)

type liveSession interface {
	SendClientContent(genai.LiveClientContentInput) error
	Receive() (*genai.LiveServerMessage, error)
	Close() error
}

type Connector func(ctx context.Context, model string, config *genai.LiveConnectConfig) (liveSession, error)

type Synthesizer struct {
	model   string
	voice   string
	connect Connector
}

func ResolveAPIKey() string {
	for _, envVar := range []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"} {
		if value := strings.TrimSpace(os.Getenv(envVar)); value != "" {
			return value
		}
	}
	return ""
}

func New(apiKey string) *Synthesizer {
	return &Synthesizer{
		model: DefaultModel,
		voice: DefaultVoice,
		connect: func(ctx context.Context, model string, config *genai.LiveConnectConfig) (liveSession, error) {
			client, err := genai.NewClient(ctx, &genai.ClientConfig{
				APIKey:  apiKey,
				Backend: genai.BackendGeminiAPI,
				HTTPOptions: genai.HTTPOptions{
					APIVersion: "v1alpha",
				},
			})
			if err != nil {
				return nil, fmt.Errorf("create genai client: %w", err)
			}

			session, err := client.Live.Connect(ctx, model, config)
			if err != nil {
				return nil, fmt.Errorf("connect live session: %w", err)
			}
			return session, nil
		},
	}
}

func (s *Synthesizer) liveConfig() *genai.LiveConnectConfig {
	return &genai.LiveConnectConfig{
		ResponseModalities: []genai.Modality{genai.ModalityAudio},
		MediaResolution:    genai.MediaResolutionMedium,
		Temperature:        genai.Ptr(float32(0)),
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				{Text: verbatimSystemPrompt},
			},
		},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: s.voice,
				},
			},
		},
	}
}

func (s *Synthesizer) Synthesize(ctx context.Context, text string, sink speech.AudioSink) error {
	config := s.liveConfig()

	session, err := s.connect(ctx, s.model, config)
	if err != nil {
		return err
	}
	defer session.Close()

	if err := session.SendClientContent(genai.LiveClientContentInput{
		Turns:        genai.Text(text),
		TurnComplete: genai.Ptr(true),
	}); err != nil {
		return fmt.Errorf("send client content: %w", err)
	}

	var playbackFormat *speech.PCMFormat

	for {
		message, err := session.Receive()
		if err != nil {
			return fmt.Errorf("receive live message: %w", err)
		}
		if message.ServerContent == nil {
			continue
		}

		if message.ServerContent.ModelTurn != nil {
			for _, part := range message.ServerContent.ModelTurn.Parts {
				if part == nil || part.InlineData == nil || len(part.InlineData.Data) == 0 {
					continue
				}

				if playbackFormat == nil {
					format, err := audio.ParsePCMFormat(part.InlineData.MIMEType)
					if err != nil {
						return err
					}
					playbackFormat = &format
				}

				if err := sink.WriteChunk(speech.AudioChunk{
					Data:   append([]byte(nil), part.InlineData.Data...),
					Format: *playbackFormat,
				}); err != nil {
					return fmt.Errorf("write audio chunk: %w", err)
				}
			}
		}

		if message.ServerContent.Interrupted {
			return fmt.Errorf("gemini interrupted audio generation")
		}

		if message.ServerContent.TurnComplete {
			break
		}
	}

	if playbackFormat == nil {
		return fmt.Errorf("gemini returned no audio data for prompt %q", strings.TrimSpace(text))
	}

	return sink.Close()
}
