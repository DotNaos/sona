package cli

import (
	"fmt"

	"github.com/DotNaos/sona/internal/audio"
	"github.com/DotNaos/sona/internal/speech"
)

type lazyPCMPlayer struct {
	player *audio.PCMPlayer
}

func (p *lazyPCMPlayer) WriteChunk(chunk speech.AudioChunk) error {
	if p.player == nil {
		player, err := audio.NewPCMPlayer(chunk.Format)
		if err != nil {
			return fmt.Errorf("create pcm player: %w", err)
		}
		p.player = player
	}
	return p.player.WriteChunk(chunk)
}

func (p *lazyPCMPlayer) Close() error {
	if p.player == nil {
		return fmt.Errorf("no audio was received from synthesizer")
	}
	return p.player.Close()
}
