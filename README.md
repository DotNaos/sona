# sona

`sona` is a macOS-first speech CLI.

- `sona say [text]` speaks text through the current speaker using Gemini native audio output.
- `sona listen` captures one spoken utterance and prints final JSON transcription using Apple SpeechTranscriber.

## Requirements

- macOS 26+
- `GEMINI_API_KEY` or `GOOGLE_API_KEY` for `sona say`
- microphone permission for `sona listen`

## Install

```bash
go install ./cmd/sona
```

## Skill Install

This repo includes a top-level `SKILL.md` and `agents/openai.yaml` so it can be used as a Codex skill source via its GitHub URL.
