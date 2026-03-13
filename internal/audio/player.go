package audio

import (
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"

	"github.com/DotNaos/sona/internal/speech"
)

type PCMPlayer struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device
	format speech.PCMFormat

	mu     sync.Mutex
	cond   *sync.Cond
	buffer []byte
	closed bool
}

func NewPCMPlayer(format speech.PCMFormat) (*PCMPlayer, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("init audio context: %w", err)
	}

	player := &PCMPlayer{
		ctx:    ctx,
		format: format,
	}
	player.cond = sync.NewCond(&player.mu)

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = format.Channels
	deviceConfig.SampleRate = format.SampleRate

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: player.onSamples,
	})
	if err != nil {
		ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("init playback device: %w", err)
	}

	player.device = device

	if err := player.device.Start(); err != nil {
		player.device.Uninit()
		player.ctx.Uninit()
		player.ctx.Free()
		return nil, fmt.Errorf("start playback device: %w", err)
	}

	return player, nil
}

func (p *PCMPlayer) WriteChunk(chunk speech.AudioChunk) error {
	if chunk.Format != p.format {
		return fmt.Errorf("pcm format changed mid-stream")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("player already closed")
	}

	p.buffer = append(p.buffer, chunk.Data...)
	p.cond.Broadcast()
	return nil
}

func (p *PCMPlayer) Close() error {
	p.mu.Lock()
	p.closed = true
	for len(p.buffer) > 0 {
		p.cond.Wait()
	}
	p.mu.Unlock()

	var err error
	if stopErr := p.device.Stop(); stopErr != nil {
		err = fmt.Errorf("stop playback device: %w", stopErr)
	}

	p.device.Uninit()
	if uninitErr := p.ctx.Uninit(); uninitErr != nil && err == nil {
		err = fmt.Errorf("uninit audio context: %w", uninitErr)
	}
	p.ctx.Free()

	return err
}

func (p *PCMPlayer) onSamples(outputSamples []byte, _ []byte, _ uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for len(p.buffer) == 0 && !p.closed {
		p.cond.Wait()
	}

	n := copy(outputSamples, p.buffer)
	if n < len(outputSamples) {
		clear(outputSamples[n:])
	}
	p.buffer = p.buffer[n:]
	if len(p.buffer) == 0 {
		p.cond.Broadcast()
	}
}
