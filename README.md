# sona

`sona` is a macOS-first speech CLI.

## Quick Start

```bash
go install github.com/DotNaos/sona/cmd/sona@latest
export GEMINI_API_KEY="your-api-key"

sona setup whispercpp

sona say "Hallo, das ist ein Test"
sona listen --locale de-DE
sona listen --locale de-DE --prompt "Bitte antworte auf Deutsch."
sona listen --backend apple --locale en-US

sona config input list
sona config input set "BuiltInMicrophoneDevice"
sona config whisper show
sona debug stt-locales
sona config show
```

- `sona say [text]` speaks text through the current speaker using Gemini native audio output.
- `sona setup whispercpp` installs `whisper-cli` via Homebrew when needed, downloads a local model into the sona config directory, and stores the resolved paths in config.
- `sona listen --locale <locale>` now uses local `whisper.cpp` by default and prints final JSON transcription to stdout.
- `sona listen --backend apple --locale <locale>` opts into Apple SpeechTranscriber explicitly.
- `sona listen --locale <locale> --prompt "..."` speaks a prompt and then immediately starts listening under the same global speech queue slot.
- `sona listen` requires an explicit `--locale` so the caller can match the spoken prompt and the expected answer language.
- `sona listen` auto-detects `whisper-cli` and the configured model path, so agents usually do not need extra STT flags after setup.
- `sona listen` plays a short start/stop indicator locally and logs the microphone device it is using to stderr.
- `sona config input list` lists all available input devices and marks the default/preferred device.
- `sona config input current` shows the current macOS default input device.
- `sona config input set <uid-or-name>` stores a preferred input device for future `sona listen` runs.
- `sona config input clear` removes the preferred device and falls back to the system default input.
- `sona config whisper show` prints the stored `whisper-cli` and model paths.
- `sona config whisper model set <path>` and `sona config whisper cli set <path>` let you override setup defaults without editing JSON manually.
- `sona debug stt-locales` shows which `SpeechTranscriber` locales are merely listed and which are actually analyzer-compatible on the current host.
- `sona config show` prints the current config file path and stored values.

## Default STT

`whispercpp` is now the default speech-to-text backend for `sona listen`.

That default is intentional:
- it is fully local and does not require a paid API
- it works reliably for non-English locales on this machine
- it avoids the current Apple `SpeechTranscriber` locale compatibility issue where many locales are listed but not actually analyzer-compatible

If you want Apple STT anyway, pass `--backend apple` explicitly.

## Requirements

- macOS 26+
- `GEMINI_API_KEY` or `GOOGLE_API_KEY` for `sona say`
- microphone permission for `sona listen`
- Homebrew for `sona setup whispercpp`, or a manual `whisper-cli` install plus a local GGML model file

## Install And Setup

Recommended path:

```bash
go install github.com/DotNaos/sona/cmd/sona@latest
sona setup whispercpp
sona listen --locale de-DE
```

What `sona setup whispercpp` does:
- installs `whisper-cpp` via `brew install whisper-cpp` if `whisper-cli` is missing
- downloads a multilingual model into `~/Library/Application Support/sona/models/`
- stores the resolved `whisper-cli` path and model path in the sona config

Manual path:

```bash
brew install whisper-cpp
sona config whisper cli set /opt/homebrew/bin/whisper-cli
sona config whisper model set /absolute/path/to/ggml-base.bin
sona listen --locale de-DE
```

Environment overrides:

```bash
export SONA_WHISPER_CLI=/opt/homebrew/bin/whisper-cli
export SONA_WHISPER_MODEL=/absolute/path/to/ggml-base.bin
sona listen --locale de-DE
```

Resolution order for `whispercpp`:
1. `--whisper-model` / `--whisper-cli`
2. `SONA_WHISPER_MODEL` / `SONA_WHISPER_CLI`
3. values stored by `sona setup whispercpp` or `sona config whisper ...`
4. auto-detected defaults in `~/Library/Application Support/sona/models/` and common binary paths

## Agent Usage

Recommended spoken question pattern:

```bash
sona listen --locale de-DE --prompt "Bitte antworte auf Deutsch."
sona listen --locale en-US --prompt "Please answer in English."
```

Agent guidance:
- Always set `--locale` to the same language as the spoken prompt, or explicitly say which language you expect back.
- Prefer plain `sona listen --locale ...` unless you specifically need to compare against Apple STT.
- Use `sona setup whispercpp` once per machine before relying on voice input in automation or agent flows.
- Use `sona debug stt-locales` before blaming the microphone when Apple STT behaves inconsistently across locales.

## Install

```bash
go install github.com/DotNaos/sona/cmd/sona@latest
```

## Skill Install

This repo includes a top-level `SKILL.md` and `agents/openai.yaml` so it can be used as a Codex skill source via its GitHub URL.
