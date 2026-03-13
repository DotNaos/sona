# sona

`sona` is a macOS-first speech CLI.

## Quick Start

```bash
go install github.com/DotNaos/sona/cmd/sona@latest
export GEMINI_API_KEY="your-api-key"

sona say "Hallo, das ist ein Test"
sona listen
sona listen --prompt "Was möchtest du sagen?"
```

- `sona say [text]` speaks text through the current speaker using Gemini native audio output.
- `sona listen` captures one spoken utterance and prints final JSON transcription using Apple SpeechTranscriber.
- `sona listen --prompt "..."` speaks a prompt and then immediately starts listening under the same global speech queue slot.

## Requirements

- macOS 26+
- `GEMINI_API_KEY` or `GOOGLE_API_KEY` for `sona say`
- microphone permission for `sona listen`

## Install

```bash
go install github.com/DotNaos/sona/cmd/sona@latest
```

## Skill Install

This repo includes a top-level `SKILL.md` and `agents/openai.yaml` so it can be used as a Codex skill source via its GitHub URL.
