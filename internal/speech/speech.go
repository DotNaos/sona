package speech

import "context"

type PCMFormat struct {
	SampleRate    uint32
	Channels      uint32
	BitsPerSample uint16
}

type AudioChunk struct {
	Data   []byte
	Format PCMFormat
}

type AudioSink interface {
	WriteChunk(chunk AudioChunk) error
	Close() error
}

type SpeechSynthesizer interface {
	Synthesize(ctx context.Context, text string, sink AudioSink) error
}

type Transcript struct {
	Text         string `json:"text"`
	Final        bool   `json:"final"`
	FinishReason string `json:"finish_reason"`
	DurationMS   int64  `json:"duration_ms"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
}

type SpeechTranscriberBackend interface {
	Listen(ctx context.Context) (Transcript, error)
}
