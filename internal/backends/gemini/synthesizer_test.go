package gemini

import "testing"

func TestLiveConfigUsesVerbatimInstruction(t *testing.T) {
	synthesizer := &Synthesizer{
		model: DefaultModel,
		voice: DefaultVoice,
	}

	config := synthesizer.liveConfig()
	if config.SystemInstruction == nil || len(config.SystemInstruction.Parts) != 1 {
		t.Fatalf("SystemInstruction not configured correctly")
	}

	if got := config.SystemInstruction.Parts[0].Text; got != verbatimSystemPrompt {
		t.Fatalf("SystemInstruction = %q, want %q", got, verbatimSystemPrompt)
	}

	if config.Temperature == nil || *config.Temperature != 0 {
		t.Fatalf("Temperature = %v, want 0", config.Temperature)
	}
}
