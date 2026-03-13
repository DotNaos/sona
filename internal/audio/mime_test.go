package audio

import "testing"

func TestParsePCMFormat(t *testing.T) {
	format, err := ParsePCMFormat("audio/L16;rate=24000;channels=1")
	if err != nil {
		t.Fatalf("ParsePCMFormat() error = %v", err)
	}
	if format.SampleRate != 24000 {
		t.Fatalf("SampleRate = %d, want 24000", format.SampleRate)
	}
	if format.Channels != 1 {
		t.Fatalf("Channels = %d, want 1", format.Channels)
	}
	if format.BitsPerSample != 16 {
		t.Fatalf("BitsPerSample = %d, want 16", format.BitsPerSample)
	}
}
