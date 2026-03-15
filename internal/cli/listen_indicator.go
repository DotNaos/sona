package cli

import (
	"fmt"
	"math"

	"github.com/DotNaos/sona/internal/audio"
	"github.com/DotNaos/sona/internal/speech"
)

const (
	listenIndicatorStart = "start"
	listenIndicatorStop  = "stop"
)

func playListenIndicator(kind string) error {
	const sampleRate = 24000
	const channels = 1
	const bitsPerSample = 16

	format := speech.PCMFormat{
		SampleRate:    sampleRate,
		Channels:      channels,
		BitsPerSample: bitsPerSample,
	}

	player, err := audio.NewPCMPlayer(format)
	if err != nil {
		return fmt.Errorf("create indicator player: %w", err)
	}
	defer player.Close()

	var chunk speech.AudioChunk
	switch kind {
	case listenIndicatorStart:
		chunk = speech.AudioChunk{Data: buildIndicatorPCM(sampleRate, []float64{880, 1320}, 0.06, 0.15), Format: format}
	case listenIndicatorStop:
		chunk = speech.AudioChunk{Data: buildIndicatorPCM(sampleRate, []float64{880, 660}, 0.07, 0.15), Format: format}
	default:
		return fmt.Errorf("unknown indicator kind %q", kind)
	}

	if err := player.WriteChunk(chunk); err != nil {
		return fmt.Errorf("write indicator audio: %w", err)
	}
	return nil
}

func buildIndicatorPCM(sampleRate uint32, tones []float64, toneDurationSec float64, volume float64) []byte {
	totalSamplesPerTone := int(float64(sampleRate) * toneDurationSec)
	data := make([]byte, 0, totalSamplesPerTone*len(tones)*2)

	for _, frequency := range tones {
		for i := 0; i < totalSamplesPerTone; i++ {
			t := float64(i) / float64(sampleRate)
			envelope := 1.0
			fadeSamples := max(1, totalSamplesPerTone/8)
			switch {
			case i < fadeSamples:
				envelope = float64(i) / float64(fadeSamples)
			case i > totalSamplesPerTone-fadeSamples:
				envelope = float64(totalSamplesPerTone-i) / float64(fadeSamples)
			}

			sample := math.Sin(2*math.Pi*frequency*t) * volume * envelope
			intSample := int16(sample * math.MaxInt16)
			data = append(data, byte(intSample), byte(intSample>>8))
		}
	}

	return data
}
