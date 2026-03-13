---
name: "sona"
description: "Use when you need local speech I/O from the terminal on macOS: `sona say` speaks text through the current speaker using Gemini native audio output, and `sona listen` records from the current microphone and returns final JSON transcription via Apple SpeechTranscriber. Use for agent voice notifications, spoken prompts, or quick one-shot microphone capture. `sona say` is globally queued across concurrent invocations so spoken output is serialized instead of overlapping."
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
- `sona listen` records one utterance and prints final JSON to stdout.
- `sona listen --prompt "What would you like to say?"` speaks the prompt and then immediately listens.

## Requirements

- `sona` must be installed in PATH.
- Install it with `go install github.com/DotNaos/sona/cmd/sona@latest`.
- `GEMINI_API_KEY` or `GOOGLE_API_KEY` must be set for `sona say`.
- `sona listen` requires macOS 26+ with Apple SpeechTranscriber available and microphone permission granted.

## Behavior Notes

- `sona say` uses a global cross-process queue, so concurrent calls are played one at a time.
- `sona listen --prompt ...` keeps the spoken prompt and the following listen phase together, so another queued `sona say` cannot interleave between them.
- The queue inserts a short pause after each spoken item to keep rapid agent notifications intelligible.
- `sona listen` prints exactly one final JSON object to stdout and sends diagnostics to stderr.

## Recommended Workflow

1. Use `sona say` for short confirmations, alerts, or user-facing spoken summaries.
2. Use `sona listen` when you need a single spoken response from the current user.
3. If `sona say` is invoked repeatedly by multiple agents, rely on the built-in queue rather than adding your own local throttling.
4. If `sona` is not yet installed, run `go install github.com/DotNaos/sona/cmd/sona@latest` before using the commands.
