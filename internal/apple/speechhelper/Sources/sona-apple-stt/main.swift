@preconcurrency import AVFAudio
import CoreMedia
import Foundation
import Speech

struct HelperOutput: Encodable {
    let text: String
    let final: Bool
    let finishReason: String
    let durationMs: Int64
}

enum HelperError: Error, LocalizedError {
    case unavailable
    case noInputFormat

    var errorDescription: String? {
        switch self {
        case .unavailable:
            return "SpeechTranscriber is not available on this system."
        case .noInputFormat:
            return "The default microphone format could not be resolved."
        }
    }
}

@main
struct SonaAppleSTT {
    static func main() async {
        do {
            let output = try await run()
            let encoder = JSONEncoder()
            encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
            let data = try encoder.encode(output)
            FileHandle.standardOutput.write(data)
            FileHandle.standardOutput.write(Data([0x0A]))
        } catch {
            FileHandle.standardError.write(Data((error.localizedDescription + "\n").utf8))
            Foundation.exit(1)
        }
    }

    static func run() async throws -> HelperOutput {
        guard SpeechTranscriber.isAvailable else {
            throw HelperError.unavailable
        }

        let locale = await SpeechTranscriber.supportedLocale(equivalentTo: .autoupdatingCurrent) ?? Locale(identifier: "en-US")
        let transcriber = SpeechTranscriber(locale: locale, preset: .progressiveTranscription)
        let analyzer = SpeechAnalyzer(modules: [transcriber])
        let engine = AVAudioEngine()
        let inputNode = engine.inputNode
        let inputFormat = inputNode.inputFormat(forBus: 0)
        guard let targetFormat = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [transcriber], considering: inputFormat) else {
            throw HelperError.noInputFormat
        }

        try await analyzer.prepareToAnalyze(in: targetFormat)
        let startedAt = Date()

        let stream = AsyncThrowingStream<AnalyzerInput, Error> { continuation in
            let converter = inputFormat == targetFormat ? nil : AVAudioConverter(from: inputFormat, to: targetFormat)
            var lastVoiceAt = Date()
            var heardSpeech = false
            var finished = false

            func finish() {
                guard !finished else { return }
                finished = true
                inputNode.removeTap(onBus: 0)
                engine.stop()
                continuation.finish()
            }

            inputNode.installTap(onBus: 0, bufferSize: 1024, format: inputFormat) { buffer, _ in
                guard !finished else { return }

                let analysisBuffer: AVAudioPCMBuffer
                do {
                    analysisBuffer = try convertBuffer(buffer, to: targetFormat, using: converter)
                } catch {
                    continuation.finish(throwing: error)
                    finish()
                    return
                }

                let level = normalizedLevel(for: analysisBuffer)
                if level > 0.015 {
                    heardSpeech = true
                    lastVoiceAt = Date()
                } else if heardSpeech, Date().timeIntervalSince(lastVoiceAt) >= 1.0 {
                    finish()
                    return
                }

                continuation.yield(AnalyzerInput(buffer: analysisBuffer))
            }

            do {
                engine.prepare()
                try engine.start()
            } catch {
                continuation.finish(throwing: error)
                finish()
                return
            }
        }

        let results = transcriber.results

        let resultTask = Task<String, Error> {
            var latestText = ""
            for try await result in results {
                latestText = String(result.text.characters)
            }
            return latestText
        }

        try await analyzer.start(inputSequence: stream)
        try await analyzer.finalizeAndFinishThroughEndOfInput()
        let latestText = try await resultTask.value
        let durationMs = Int64(Date().timeIntervalSince(startedAt) * 1000)

        return HelperOutput(
            text: latestText.trimmingCharacters(in: .whitespacesAndNewlines),
            final: true,
            finishReason: "vad_silence",
            durationMs: durationMs
        )
    }

    static func convertBuffer(_ buffer: AVAudioPCMBuffer, to targetFormat: AVAudioFormat, using converter: AVAudioConverter?) throws -> AVAudioPCMBuffer {
        guard let converter else {
            return buffer
        }

        let ratio = targetFormat.sampleRate / buffer.format.sampleRate
        let capacity = AVAudioFrameCount(Double(buffer.frameLength) * ratio) + 1
        guard let converted = AVAudioPCMBuffer(pcmFormat: targetFormat, frameCapacity: capacity) else {
            return buffer
        }

        var sourceConsumed = false
        var conversionError: NSError?
        converter.convert(to: converted, error: &conversionError) { _, outStatus in
            if sourceConsumed {
                outStatus.pointee = .endOfStream
                return nil
            }
            sourceConsumed = true
            outStatus.pointee = .haveData
            return buffer
        }

        if let conversionError {
            throw conversionError
        }

        return converted
    }

    static func normalizedLevel(for buffer: AVAudioPCMBuffer) -> Float {
        let frameLength = Int(buffer.frameLength)
        guard frameLength > 0 else { return 0 }

        switch buffer.format.commonFormat {
        case .pcmFormatFloat32:
            guard let channelData = buffer.floatChannelData else { return 0 }
            let samples = channelData[0]
            var sum: Float = 0
            for i in 0..<frameLength {
                let value = samples[i]
                sum += value * value
            }
            return sqrt(sum / Float(frameLength))
        case .pcmFormatInt16:
            guard let channelData = buffer.int16ChannelData else { return 0 }
            let samples = channelData[0]
            var sum: Float = 0
            for i in 0..<frameLength {
                let value = Float(samples[i]) / Float(Int16.max)
                sum += value * value
            }
            return sqrt(sum / Float(frameLength))
        default:
            return 0
        }
    }
}
