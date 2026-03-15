---
name: "sona"
description: "Use when you need local speech I/O from the terminal on macOS: `sona say` speaks text through the current speaker using Gemini native audio output, and `sona listen` records from the current microphone and returns final JSON transcription using local whisper.cpp by default. Use for agent voice notifications, spoken prompts, or quick one-shot microphone capture. `sona say` is globally queued across concurrent invocations so spoken output is serialized instead of overlapping."
---

# Sona

## When To Use

Use this skill when a task needs local voice input or output from the terminal on macOS.

- Speak short text aloud for user-visible feedback.
- Capture one spoken utterance and hand the transcript back to the agent as JSON.
- Avoid overlapping speech from multiple concurrent agent invocations.

## Commands

- `sona say "Hello"` speaks the given text aloud.
- `echo "Hello" | sona say` reads text from stdin when no positional text is provided.
- `sona setup whispercpp` installs or configures the default local STT backend on the current machine.
- `sona listen --locale de-DE` records one utterance and prints final JSON to stdout using `whispercpp` by default.
- `sona listen --locale de-DE --prompt "Bitte antworte auf Deutsch."` speaks the prompt and then immediately listens.
- `sona listen --backend apple --locale en-US` uses Apple SpeechTranscriber explicitly.
- `sona config whisper show` prints the configured `whisper-cli` and model paths.

## Requirements

- `sona` must be installed in PATH.
- Install it with `go install github.com/DotNaos/sona/cmd/sona@latest`.
- `GEMINI_API_KEY` or `GOOGLE_API_KEY` must be set for `sona say`.
- `sona listen` requires an explicit `--locale`, so the caller states which language it expects back.
- `sona listen` defaults to local `whispercpp`, which needs `whisper-cli` plus a GGML model file.
- `sona setup whispercpp` is the recommended one-time machine setup path.
- `sona listen --backend apple` requires macOS 26+ with Apple SpeechTranscriber available and microphone permission granted.

## Behavior Notes

- `sona say` uses a global cross-process queue, so concurrent calls are played one at a time.
- `sona listen --prompt ...` keeps the spoken prompt and the following listen phase together, so another queued `sona say` cannot interleave between them.
- If the spoken language is German, write normal German spelling with `ä`, `ö`, `ü`, and `ß` instead of `ae`, `oe`, `ue`, or `ss` when possible, because the TTS pronunciation is noticeably more natural that way.
- After `sona setup whispercpp`, most agents should only need `sona listen --locale ...`.
- The queue inserts a short pause after each spoken item to keep rapid agent notifications intelligible.
- `sona listen` prints exactly one final JSON object to stdout and sends diagnostics to stderr.

## Recommended Workflow

1. Use `sona say` for short confirmations, alerts, or user-facing spoken summaries.
2. Run `sona setup whispercpp` once per machine before relying on voice input.
3. Use `sona listen --locale ...` when you need a single spoken response from the current user.
4. Set `--locale` to the same language as your spoken prompt, or explicitly say which language you expect in the answer.
5. For German prompts or spoken summaries, prefer proper umlauts and `ß` in the text you pass to `sona say`.
6. If `sona say` is invoked repeatedly by multiple agents, rely on the built-in queue rather than adding your own local throttling.
7. If `sona` is not yet installed, run `go install github.com/DotNaos/sona/cmd/sona@latest` before using the commands.
