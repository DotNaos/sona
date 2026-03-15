@preconcurrency import AVFAudio
import AudioToolbox
import CoreAudio
import CoreMedia
import Foundation
import Speech

struct HelperOutput: Encodable, Sendable {
    let text: String
    let final: Bool
    let finishReason: String
    let durationMs: Int64

    enum CodingKeys: String, CodingKey {
        case text
        case final
        case finishReason = "finish_reason"
        case durationMs = "duration_ms"
    }
}

struct InputDeviceOutput: Encodable, Sendable {
    let name: String
    let uid: String
    let isDefault: Bool

    enum CodingKeys: String, CodingKey {
        case name
        case uid
        case isDefault = "is_default"
    }
}

struct ListenCommand {
    let preferredDeviceUID: String?
    let localeIdentifier: String?
    let logDeviceSelection: Bool
    let debug: Bool
}

struct RawInputCommand {
    let preferredDeviceUID: String?
    let logDeviceSelection: Bool
    let debug: Bool
    let seconds: Int
}

struct CaptureUtteranceCommand {
    let preferredDeviceUID: String?
    let logDeviceSelection: Bool
    let debug: Bool
}

struct STTLocalesCommand {
    let preferredDeviceUID: String?
    let logDeviceSelection: Bool
}

struct STTLocaleOutput: Encodable, Sendable {
    let identifier: String
    let analyzerCompatible: Bool

    enum CodingKeys: String, CodingKey {
        case identifier
        case analyzerCompatible = "analyzer_compatible"
    }
}

struct STTLocalesOutput: Encodable, Sendable {
    let inputDevice: InputDeviceOutput
    let inputSampleRate: Double
    let inputChannels: UInt32
    let locales: [STTLocaleOutput]

    enum CodingKeys: String, CodingKey {
        case inputDevice = "input_device"
        case inputSampleRate = "input_sample_rate"
        case inputChannels = "input_channels"
        case locales
    }
}

struct CaptureUtteranceOutput: Encodable, Sendable {
    let audioPath: String
    let durationMs: Int64
    let finishReason: String
    let inputDevice: InputDeviceOutput

    enum CodingKeys: String, CodingKey {
        case audioPath = "audio_path"
        case durationMs = "duration_ms"
        case finishReason = "finish_reason"
        case inputDevice = "input_device"
    }
}

enum HelperCommand {
    case listen(ListenCommand)
    case listInputs
    case currentInput
    case rawInput(RawInputCommand)
    case captureUtterance(CaptureUtteranceCommand)
    case sttLocales(STTLocalesCommand)
}

final class SessionState: @unchecked Sendable {
    private let lock = NSLock()
    private var latestText = ""
    private var finishReason = "vad_silence"
    private var finished = false

    func updateLatestText(_ text: String) {
        lock.lock()
        latestText = text
        lock.unlock()
    }

    func hasNonEmptyText() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        return !latestText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    func markFinished(reason: String) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        guard !finished else { return false }
        finished = true
        finishReason = reason
        return true
    }

    func snapshot() -> (text: String, finishReason: String) {
        lock.lock()
        defer { lock.unlock() }
        return (latestText, finishReason)
    }
}

enum DebugLog {
    static func write(_ message: String) {
        FileHandle.standardError.write(Data(("[sona-apple-stt] " + message + "\n").utf8))
    }
}

struct InputDeviceInfo {
    let id: AudioDeviceID
    let name: String
    let uid: String
    let isDefault: Bool
}

struct RawInputDebugOutput: Encodable, Sendable {
    let inputDevice: InputDeviceOutput
    let durationMs: Int64
    let buffers: Int
    let nonSilentBuffers: Int
    let averageLevel: Double
    let maxLevel: Double

    enum CodingKeys: String, CodingKey {
        case inputDevice = "input_device"
        case durationMs = "duration_ms"
        case buffers
        case nonSilentBuffers = "non_silent_buffers"
        case averageLevel = "average_level"
        case maxLevel = "max_level"
    }
}

final class RawCaptureState: @unchecked Sendable {
    private let lock = NSLock()
    private var buffers = 0
    private var nonSilentBuffers = 0
    private var totalLevel: Double = 0
    private var maxLevel: Double = 0

    func record(level: Float, threshold: Float) {
        lock.lock()
        defer { lock.unlock() }
        buffers += 1
        let value = Double(level)
        totalLevel += value
        if value > maxLevel {
            maxLevel = value
        }
        if level > threshold {
            nonSilentBuffers += 1
        }
    }

    func snapshot() -> (buffers: Int, nonSilentBuffers: Int, averageLevel: Double, maxLevel: Double) {
        lock.lock()
        defer { lock.unlock() }
        let averageLevel = buffers == 0 ? 0 : totalLevel / Double(buffers)
        return (buffers, nonSilentBuffers, averageLevel, maxLevel)
    }
}

final class CompletionBox<T: Sendable>: @unchecked Sendable {
    private let lock = NSLock()
    private var finished = false
    private let continuation: CheckedContinuation<T, Error>

    init(_ continuation: CheckedContinuation<T, Error>) {
        self.continuation = continuation
    }

    func succeed(_ value: T) {
        lock.lock()
        defer { lock.unlock() }
        guard !finished else { return }
        finished = true
        continuation.resume(returning: value)
    }

    func fail(_ error: Error) {
        lock.lock()
        defer { lock.unlock() }
        guard !finished else { return }
        finished = true
        continuation.resume(throwing: error)
    }
}

enum HelperError: Error, LocalizedError {
    case unavailable
    case noInputFormat
    case localeNoInputFormat(String)
    case outputFileUnavailable
    case unknownCommand(String)
    case missingValue(String)
    case unsupportedArgument(String)
    case currentInputUnavailable
    case deviceNotFound(String)
    case audioUnitUnavailable
    case missingStringProperty(String)
    case coreAudio(String, OSStatus)
    case audioUnitSetDevice(OSStatus)

    var errorDescription: String? {
        switch self {
        case .unavailable:
            return "SpeechTranscriber is not available on this system."
        case .noInputFormat:
            return "The selected microphone format could not be resolved."
        case .localeNoInputFormat(let locale):
            return "The locale \(locale) is recognized, but this system could not produce a compatible SpeechAnalyzer input format for it. Try `sona debug stt-locales` to inspect which locales actually work on this host."
        case .outputFileUnavailable:
            return "The capture output file could not be created."
        case .unknownCommand(let command):
            return "Unknown helper command: \(command)"
        case .missingValue(let flag):
            return "Missing value for \(flag)."
        case .unsupportedArgument(let argument):
            return "Unsupported argument: \(argument)"
        case .currentInputUnavailable:
            return "The current input device could not be resolved."
        case .deviceNotFound(let uid):
            return "No input device matches uid \(uid)."
        case .audioUnitUnavailable:
            return "The input node audio unit is not available."
        case .missingStringProperty(let property):
            return "The audio property \(property) returned no string value."
        case .coreAudio(let operation, let status):
            return "\(operation) failed with OSStatus \(status)."
        case .audioUnitSetDevice(let status):
            return "Setting the input device failed with OSStatus \(status)."
        }
    }
}

@main
struct SonaAppleSTT {
    static func main() async {
        do {
            let command = try parseCommandLine(Array(CommandLine.arguments.dropFirst()))
            switch command {
            case .listen(let listenCommand):
                let output = try await runListen(command: listenCommand)
                try writeJSON(output)
            case .listInputs:
                let devices = try listInputDevices().map {
                    InputDeviceOutput(name: $0.name, uid: $0.uid, isDefault: $0.isDefault)
                }
                try writeJSON(devices)
            case .currentInput:
                let device = try currentInputDevice()
                try writeJSON(InputDeviceOutput(name: device.name, uid: device.uid, isDefault: true))
            case .rawInput(let rawInputCommand):
                let output = try await runRawInput(command: rawInputCommand)
                try writeJSON(output)
            case .captureUtterance(let captureUtteranceCommand):
                let output = try await runCaptureUtterance(command: captureUtteranceCommand)
                try writeJSON(output)
            case .sttLocales(let sttLocalesCommand):
                let output = try await runSTTLocales(command: sttLocalesCommand)
                try writeJSON(output)
            }
        } catch {
            FileHandle.standardError.write(Data((error.localizedDescription + "\n").utf8))
            Foundation.exit(1)
        }
    }

    static func parseCommandLine(_ args: [String]) throws -> HelperCommand {
        if args.isEmpty {
            return .listen(ListenCommand(preferredDeviceUID: nil, localeIdentifier: nil, logDeviceSelection: false, debug: false))
        }

        let command = args[0]
        let flags = Array(args.dropFirst())

        switch command {
        case "listen":
            var preferredDeviceUID: String?
            var localeIdentifier: String?
            var logDeviceSelection = false
            var debug = false
            var index = 0
            while index < flags.count {
                switch flags[index] {
                case "--device-uid":
                    index += 1
                    guard index < flags.count else {
                        throw HelperError.missingValue("--device-uid")
                    }
                    preferredDeviceUID = flags[index]
                case "--locale":
                    index += 1
                    guard index < flags.count else {
                        throw HelperError.missingValue("--locale")
                    }
                    localeIdentifier = flags[index]
                case "--log-device":
                    logDeviceSelection = true
                case "--debug":
                    debug = true
                default:
                    throw HelperError.unsupportedArgument(flags[index])
                }
                index += 1
            }
            return .listen(ListenCommand(
                preferredDeviceUID: preferredDeviceUID,
                localeIdentifier: localeIdentifier,
                logDeviceSelection: logDeviceSelection,
                debug: debug
            ))
        case "list-inputs":
            guard flags.isEmpty else {
                throw HelperError.unsupportedArgument(flags[0])
            }
            return .listInputs
        case "current-input":
            guard flags.isEmpty else {
                throw HelperError.unsupportedArgument(flags[0])
            }
            return .currentInput
        case "raw-input":
            var preferredDeviceUID: String?
            var logDeviceSelection = false
            var debug = false
            var seconds = 3
            var index = 0
            while index < flags.count {
                switch flags[index] {
                case "--device-uid":
                    index += 1
                    guard index < flags.count else {
                        throw HelperError.missingValue("--device-uid")
                    }
                    preferredDeviceUID = flags[index]
                case "--log-device":
                    logDeviceSelection = true
                case "--debug":
                    debug = true
                case "--seconds":
                    index += 1
                    guard index < flags.count else {
                        throw HelperError.missingValue("--seconds")
                    }
                    guard let parsedSeconds = Int(flags[index]), parsedSeconds > 0 else {
                        throw HelperError.unsupportedArgument(flags[index])
                    }
                    seconds = parsedSeconds
                default:
                    throw HelperError.unsupportedArgument(flags[index])
                }
                index += 1
            }
            return .rawInput(RawInputCommand(
                preferredDeviceUID: preferredDeviceUID,
                logDeviceSelection: logDeviceSelection,
                debug: debug,
                seconds: seconds
            ))
        case "capture-utterance":
            var preferredDeviceUID: String?
            var logDeviceSelection = false
            var debug = false
            var index = 0
            while index < flags.count {
                switch flags[index] {
                case "--device-uid":
                    index += 1
                    guard index < flags.count else {
                        throw HelperError.missingValue("--device-uid")
                    }
                    preferredDeviceUID = flags[index]
                case "--log-device":
                    logDeviceSelection = true
                case "--debug":
                    debug = true
                default:
                    throw HelperError.unsupportedArgument(flags[index])
                }
                index += 1
            }
            return .captureUtterance(CaptureUtteranceCommand(
                preferredDeviceUID: preferredDeviceUID,
                logDeviceSelection: logDeviceSelection,
                debug: debug
            ))
        case "stt-locales":
            var preferredDeviceUID: String?
            var logDeviceSelection = false
            var index = 0
            while index < flags.count {
                switch flags[index] {
                case "--device-uid":
                    index += 1
                    guard index < flags.count else {
                        throw HelperError.missingValue("--device-uid")
                    }
                    preferredDeviceUID = flags[index]
                case "--log-device":
                    logDeviceSelection = true
                default:
                    throw HelperError.unsupportedArgument(flags[index])
                }
                index += 1
            }
            return .sttLocales(STTLocalesCommand(
                preferredDeviceUID: preferredDeviceUID,
                logDeviceSelection: logDeviceSelection
            ))
        default:
            throw HelperError.unknownCommand(command)
        }
    }

    static func writeJSON<T: Encodable>(_ value: T) throws {
        let encoder = JSONEncoder()
        encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
        let data = try encoder.encode(value)
        FileHandle.standardOutput.write(data)
        FileHandle.standardOutput.write(Data([0x0A]))
    }

    static func runListen(command: ListenCommand) async throws -> HelperOutput {
        let postSpeechSilenceTimeout: TimeInterval = 1.0
        let levelThreshold: Float = 0.015

        guard SpeechTranscriber.isAvailable else {
            throw HelperError.unavailable
        }

        let requestedLocale = command.localeIdentifier?.trimmingCharacters(in: .whitespacesAndNewlines)
        let locale = await resolvedLocale(requestedLocale) ?? Locale(identifier: "en-US")
        let transcriber = SpeechTranscriber(
            locale: locale,
            transcriptionOptions: [],
            reportingOptions: [.volatileResults],
            attributeOptions: []
        )
        let analyzer = SpeechAnalyzer(modules: [transcriber])
        let engine = AVAudioEngine()
        let inputNode = engine.inputNode

        let selectedDevice: InputDeviceInfo
        if let preferredUID = command.preferredDeviceUID?.trimmingCharacters(in: .whitespacesAndNewlines), !preferredUID.isEmpty {
            selectedDevice = try inputDevice(uid: preferredUID)
            try setInputDevice(selectedDevice.id, on: inputNode)
        } else {
            selectedDevice = try currentInputDevice()
        }

        if command.logDeviceSelection {
            let message = "Listening with input device: \(selectedDevice.name) [\(selectedDevice.uid)]" + (selectedDevice.isDefault ? " (system default)\n" : "\n")
            FileHandle.standardError.write(Data(message.utf8))
        }

        let inputFormat = inputNode.inputFormat(forBus: 0)
        guard let targetFormat = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [transcriber], considering: inputFormat) else {
            throw HelperError.localeNoInputFormat(locale.identifier)
        }

        if command.debug {
            DebugLog.write("debug listen enabled")
            DebugLog.write("speech transcriber requested_locale=\(requestedLocale ?? "nil") resolved_locale=\(locale.identifier)")
            DebugLog.write("input format sampleRate=\(inputFormat.sampleRate) channels=\(inputFormat.channelCount) commonFormat=\(inputFormat.commonFormat.rawValue)")
            DebugLog.write("target format sampleRate=\(targetFormat.sampleRate) channels=\(targetFormat.channelCount) commonFormat=\(targetFormat.commonFormat.rawValue)")
            DebugLog.write("silence threshold level=\(levelThreshold)")
        }

        try await analyzer.prepareToAnalyze(in: targetFormat)
        let startedAt = Date()
        let sessionState = SessionState()
        var finishListening: ((String) -> Void)?

        let stream = AsyncThrowingStream<AnalyzerInput, Error> { continuation in
            var lastVoiceAt = Date()
            var finished = false
            var lastDebugAt = Date.distantPast
            var observedSpeech = false

            func finish(reason: String) {
                guard !finished else { return }
                finished = true
                if command.debug {
                    let snapshot = sessionState.snapshot()
                    DebugLog.write("finishing listen reason=\(reason) text=\(snapshot.text.debugDescription)")
                }
                _ = sessionState.markFinished(reason: reason)
                inputNode.removeTap(onBus: 0)
                engine.stop()
                continuation.finish()
            }

            finishListening = finish

            inputNode.installTap(onBus: 0, bufferSize: 1024, format: inputFormat) { buffer, _ in
                guard !finished else { return }

                let rawLevel = normalizedLevel(for: buffer)
                let analysisBuffer: AVAudioPCMBuffer
                do {
                    analysisBuffer = try convertBuffer(buffer, to: targetFormat)
                } catch {
                    continuation.finish(throwing: error)
                    finish(reason: "conversion_error")
                    return
                }

                let level = normalizedLevel(for: analysisBuffer)
                let now = Date()

                if command.debug, now.timeIntervalSince(lastDebugAt) >= 0.75 {
                    lastDebugAt = now
                    DebugLog.write(String(
                        format: "audio raw_level=%.5f converted_level=%.5f converted_frames=%u text_nonempty=%@",
                        rawLevel,
                        level,
                        analysisBuffer.frameLength,
                        sessionState.hasNonEmptyText() ? "yes" : "no"
                    ))
                }

                if level > levelThreshold {
                    lastVoiceAt = Date()
                    if command.debug, !observedSpeech {
                        observedSpeech = true
                        DebugLog.write(String(format: "voice activity detected level=%.5f", level))
                    }
                } else if sessionState.hasNonEmptyText(), Date().timeIntervalSince(lastVoiceAt) >= postSpeechSilenceTimeout {
                    if command.debug {
                        DebugLog.write(String(format: "silence after text for %.2fs", Date().timeIntervalSince(lastVoiceAt)))
                    }
                    finish(reason: "vad_silence")
                    return
                }

                continuation.yield(AnalyzerInput(buffer: analysisBuffer))
            }

            do {
                engine.prepare()
                try engine.start()
                if command.debug {
                    DebugLog.write("audio engine started")
                }
            } catch {
                continuation.finish(throwing: error)
                finish(reason: "engine_start_error")
            }
        }

        let results = transcriber.results
        let resultTask = Task<String, Error> {
            var latestText = ""
            for try await result in results {
                latestText = String(result.text.characters)
                sessionState.updateLatestText(latestText)
                if command.debug {
                    DebugLog.write("transcriber result final=\(result.isFinal) text=\(latestText.debugDescription)")
                }
                if result.isFinal {
                    finishListening?("final_result")
                }
            }
            if command.debug {
                DebugLog.write("transcriber result stream ended")
            }
            return latestText
        }

        try await analyzer.start(inputSequence: stream)
        try await analyzer.finalizeAndFinishThroughEndOfInput()
        _ = try await resultTask.value
        let snapshot = sessionState.snapshot()
        let durationMs = Int64(Date().timeIntervalSince(startedAt) * 1000)

        return HelperOutput(
            text: snapshot.text.trimmingCharacters(in: .whitespacesAndNewlines),
            final: true,
            finishReason: snapshot.finishReason,
            durationMs: durationMs
        )
    }

    static func runRawInput(command: RawInputCommand) async throws -> RawInputDebugOutput {
        let levelThreshold: Float = 0.015
        let engine = AVAudioEngine()
        let inputNode = engine.inputNode

        let selectedDevice: InputDeviceInfo
        if let preferredUID = command.preferredDeviceUID?.trimmingCharacters(in: .whitespacesAndNewlines), !preferredUID.isEmpty {
            selectedDevice = try inputDevice(uid: preferredUID)
            try setInputDevice(selectedDevice.id, on: inputNode)
        } else {
            selectedDevice = try currentInputDevice()
        }

        if command.logDeviceSelection {
            let message = "Raw input test with device: \(selectedDevice.name) [\(selectedDevice.uid)]" + (selectedDevice.isDefault ? " (system default)\n" : "\n")
            FileHandle.standardError.write(Data(message.utf8))
        }

        let inputFormat = inputNode.inputFormat(forBus: 0)
        if command.debug {
            DebugLog.write("raw input debug enabled")
            DebugLog.write("input format sampleRate=\(inputFormat.sampleRate) channels=\(inputFormat.channelCount) commonFormat=\(inputFormat.commonFormat.rawValue)")
            DebugLog.write("raw capture duration=\(command.seconds)s threshold=\(levelThreshold)")
        }

        let state = RawCaptureState()
        let startedAt = Date()

        return try await withCheckedThrowingContinuation { continuation in
            let completion = CompletionBox<RawInputDebugOutput>(continuation)
            var lastDebugAt = Date.distantPast

            func finishSuccessfully() {
                inputNode.removeTap(onBus: 0)
                engine.stop()
                let snapshot = state.snapshot()
                let durationMs = Int64(Date().timeIntervalSince(startedAt) * 1000)
                let output = RawInputDebugOutput(
                    inputDevice: InputDeviceOutput(name: selectedDevice.name, uid: selectedDevice.uid, isDefault: selectedDevice.isDefault),
                    durationMs: durationMs,
                    buffers: snapshot.buffers,
                    nonSilentBuffers: snapshot.nonSilentBuffers,
                    averageLevel: snapshot.averageLevel,
                    maxLevel: snapshot.maxLevel
                )
                if command.debug {
                    DebugLog.write("raw capture finished buffers=\(snapshot.buffers) nonSilentBuffers=\(snapshot.nonSilentBuffers) averageLevel=\(snapshot.averageLevel) maxLevel=\(snapshot.maxLevel)")
                }
                completion.succeed(output)
            }

            inputNode.installTap(onBus: 0, bufferSize: 1024, format: inputFormat) { buffer, _ in
                let level = normalizedLevel(for: buffer)
                state.record(level: level, threshold: levelThreshold)

                if command.debug {
                    let now = Date()
                    if now.timeIntervalSince(lastDebugAt) >= 0.5 {
                        lastDebugAt = now
                        DebugLog.write(String(format: "raw audio level=%.5f", level))
                    }
                }
            }

            do {
                engine.prepare()
                try engine.start()
                if command.debug {
                    DebugLog.write("raw audio engine started")
                }
            } catch {
                inputNode.removeTap(onBus: 0)
                engine.stop()
                completion.fail(error)
                return
            }

            Task {
                try? await Task.sleep(nanoseconds: UInt64(command.seconds) * 1_000_000_000)
                finishSuccessfully()
            }
        }
    }

    static func runSTTLocales(command: STTLocalesCommand) async throws -> STTLocalesOutput {
        guard SpeechTranscriber.isAvailable else {
            throw HelperError.unavailable
        }

        let engine = AVAudioEngine()
        let inputNode = engine.inputNode

        let selectedDevice: InputDeviceInfo
        if let preferredUID = command.preferredDeviceUID?.trimmingCharacters(in: .whitespacesAndNewlines), !preferredUID.isEmpty {
            selectedDevice = try inputDevice(uid: preferredUID)
            try setInputDevice(selectedDevice.id, on: inputNode)
        } else {
            selectedDevice = try currentInputDevice()
        }

        if command.logDeviceSelection {
            let message = "STT locale check with device: \(selectedDevice.name) [\(selectedDevice.uid)]" + (selectedDevice.isDefault ? " (system default)\n" : "\n")
            FileHandle.standardError.write(Data(message.utf8))
        }

        let inputFormat = inputNode.inputFormat(forBus: 0)
        let locales = await SpeechTranscriber.supportedLocales
        let outputs = await withTaskGroup(of: STTLocaleOutput.self) { group in
            for locale in locales {
                group.addTask {
                    let transcriber = SpeechTranscriber(
                        locale: locale,
                        transcriptionOptions: [],
                        reportingOptions: [.volatileResults],
                        attributeOptions: []
                    )
                    let targetFormat = await SpeechAnalyzer.bestAvailableAudioFormat(compatibleWith: [transcriber], considering: inputFormat)
                    return STTLocaleOutput(identifier: locale.identifier, analyzerCompatible: targetFormat != nil)
                }
            }

            var items: [STTLocaleOutput] = []
            for await item in group {
                items.append(item)
            }
            return items.sorted { lhs, rhs in
                if lhs.analyzerCompatible != rhs.analyzerCompatible {
                    return lhs.analyzerCompatible && !rhs.analyzerCompatible
                }
                return lhs.identifier.localizedCaseInsensitiveCompare(rhs.identifier) == .orderedAscending
            }
        }

        return STTLocalesOutput(
            inputDevice: InputDeviceOutput(name: selectedDevice.name, uid: selectedDevice.uid, isDefault: selectedDevice.isDefault),
            inputSampleRate: inputFormat.sampleRate,
            inputChannels: inputFormat.channelCount,
            locales: outputs
        )
    }

    static func runCaptureUtterance(command: CaptureUtteranceCommand) async throws -> CaptureUtteranceOutput {
        let levelThreshold: Float = 0.015
        let noSpeechTimeout: TimeInterval = 15.0
        let postSpeechSilenceTimeout: TimeInterval = 1.0
        let maxCaptureDuration: TimeInterval = 30.0

        let engine = AVAudioEngine()
        let inputNode = engine.inputNode

        let selectedDevice: InputDeviceInfo
        if let preferredUID = command.preferredDeviceUID?.trimmingCharacters(in: .whitespacesAndNewlines), !preferredUID.isEmpty {
            selectedDevice = try inputDevice(uid: preferredUID)
            try setInputDevice(selectedDevice.id, on: inputNode)
        } else {
            selectedDevice = try currentInputDevice()
        }

        if command.logDeviceSelection {
            let message = "Capture utterance with device: \(selectedDevice.name) [\(selectedDevice.uid)]" + (selectedDevice.isDefault ? " (system default)\n" : "\n")
            FileHandle.standardError.write(Data(message.utf8))
        }

        let inputFormat = inputNode.inputFormat(forBus: 0)
        guard let targetFormat = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: 16_000, channels: 1, interleaved: false) else {
            throw HelperError.noInputFormat
        }

        let outputURL = URL(fileURLWithPath: NSTemporaryDirectory())
            .appendingPathComponent("sona-capture-\(UUID().uuidString).wav")
        guard let outputFile = try? AVAudioFile(
            forWriting: outputURL,
            settings: targetFormat.settings,
            commonFormat: .pcmFormatInt16,
            interleaved: false
        ) else {
            throw HelperError.outputFileUnavailable
        }

        if command.debug {
            DebugLog.write("capture utterance enabled")
            DebugLog.write("input format sampleRate=\(inputFormat.sampleRate) channels=\(inputFormat.channelCount) commonFormat=\(inputFormat.commonFormat.rawValue)")
            DebugLog.write("capture target sampleRate=\(targetFormat.sampleRate) channels=\(targetFormat.channelCount) commonFormat=\(targetFormat.commonFormat.rawValue)")
        }

        let startedAt = Date()

        return try await withCheckedThrowingContinuation { continuation in
            let completion = CompletionBox<CaptureUtteranceOutput>(continuation)
            var lastVoiceAt = startedAt
            var lastDebugAt = Date.distantPast
            var observedSpeech = false
            var finished = false
            var recordedFrames: AVAudioFramePosition = 0

            func finish(reason: String) {
                guard !finished else { return }
                finished = true
                inputNode.removeTap(onBus: 0)
                engine.stop()

                let durationMs = Int64(Double(recordedFrames) / targetFormat.sampleRate * 1000.0)
                if !observedSpeech {
                    try? FileManager.default.removeItem(at: outputURL)
                }
                completion.succeed(CaptureUtteranceOutput(
                    audioPath: observedSpeech ? outputURL.path : "",
                    durationMs: durationMs,
                    finishReason: reason,
                    inputDevice: InputDeviceOutput(name: selectedDevice.name, uid: selectedDevice.uid, isDefault: selectedDevice.isDefault)
                ))
            }

            inputNode.installTap(onBus: 0, bufferSize: 1024, format: inputFormat) { buffer, _ in
                guard !finished else { return }

                let now = Date()

                let converted: AVAudioPCMBuffer
                do {
                    converted = try convertBuffer(buffer, to: targetFormat)
                } catch {
                    inputNode.removeTap(onBus: 0)
                    engine.stop()
                    try? FileManager.default.removeItem(at: outputURL)
                    completion.fail(error)
                    finished = true
                    return
                }

                let level = normalizedLevel(for: converted)
                if command.debug, now.timeIntervalSince(lastDebugAt) >= 0.75 {
                    lastDebugAt = now
                    DebugLog.write(String(
                        format: "capture audio level=%.5f frames=%u speech_started=%@",
                        level,
                        converted.frameLength,
                        observedSpeech ? "yes" : "no"
                    ))
                }

                if level > levelThreshold {
                    observedSpeech = true
                    lastVoiceAt = now
                    if command.debug {
                        DebugLog.write(String(format: "capture voice activity level=%.5f", level))
                    }
                }

                if observedSpeech {
                    do {
                        try outputFile.write(from: converted)
                        recordedFrames += AVAudioFramePosition(converted.frameLength)
                    } catch {
                        inputNode.removeTap(onBus: 0)
                        engine.stop()
                        try? FileManager.default.removeItem(at: outputURL)
                        completion.fail(error)
                        finished = true
                        return
                    }
                } else if now.timeIntervalSince(startedAt) >= noSpeechTimeout {
                    finish(reason: "no_speech_timeout")
                    return
                }

                if observedSpeech && now.timeIntervalSince(lastVoiceAt) >= postSpeechSilenceTimeout {
                    finish(reason: "vad_silence")
                    return
                }

                if now.timeIntervalSince(startedAt) >= maxCaptureDuration {
                    finish(reason: observedSpeech ? "max_duration" : "no_speech_timeout")
                }
            }

            do {
                engine.prepare()
                try engine.start()
                if command.debug {
                    DebugLog.write("capture audio engine started")
                }
            } catch {
                inputNode.removeTap(onBus: 0)
                engine.stop()
                try? FileManager.default.removeItem(at: outputURL)
                completion.fail(error)
            }
        }
    }

    static func resolvedLocale(_ identifier: String?) async -> Locale? {
        let supportedLocales = await SpeechTranscriber.supportedLocales

        if let identifier {
            let requested = Locale(identifier: identifier)
            if let supported = await SpeechTranscriber.supportedLocale(equivalentTo: requested) {
                return supported
            }

            if let languageCode = requested.language.languageCode?.identifier,
               let languageMatch = supportedLocales.first(where: { $0.language.languageCode?.identifier == languageCode }) {
                return languageMatch
            }
        }

        if let current = await SpeechTranscriber.supportedLocale(equivalentTo: .autoupdatingCurrent) {
            return current
        }
        if let english = await SpeechTranscriber.supportedLocale(equivalentTo: Locale(identifier: "en-US")) {
            return english
        }
        return supportedLocales.first
    }

    static func listInputDevices() throws -> [InputDeviceInfo] {
        let defaultInputID = try defaultInputDeviceID()
        return try allAudioDeviceIDs().compactMap { deviceID in
            guard try isInputDevice(deviceID) else { return nil }
            let name = try deviceName(deviceID)
            let uid = try deviceUID(deviceID)
            return InputDeviceInfo(id: deviceID, name: name, uid: uid, isDefault: deviceID == defaultInputID)
        }
        .sorted { lhs, rhs in
            if lhs.isDefault != rhs.isDefault {
                return lhs.isDefault && !rhs.isDefault
            }
            return lhs.name.localizedCaseInsensitiveCompare(rhs.name) == .orderedAscending
        }
    }

    static func currentInputDevice() throws -> InputDeviceInfo {
        let defaultInputID = try defaultInputDeviceID()
        guard defaultInputID != kAudioObjectUnknown else {
            throw HelperError.currentInputUnavailable
        }
        return InputDeviceInfo(
            id: defaultInputID,
            name: try deviceName(defaultInputID),
            uid: try deviceUID(defaultInputID),
            isDefault: true
        )
    }

    static func inputDevice(uid: String) throws -> InputDeviceInfo {
        guard let match = try listInputDevices().first(where: { $0.uid == uid }) else {
            throw HelperError.deviceNotFound(uid)
        }
        return match
    }

    static func setInputDevice(_ deviceID: AudioDeviceID, on inputNode: AVAudioInputNode) throws {
        guard let audioUnit = inputNode.audioUnit else {
            throw HelperError.audioUnitUnavailable
        }

        var mutableDeviceID = deviceID
        let status = AudioUnitSetProperty(
            audioUnit,
            kAudioOutputUnitProperty_CurrentDevice,
            kAudioUnitScope_Global,
            0,
            &mutableDeviceID,
            UInt32(MemoryLayout<AudioDeviceID>.size)
        )
        guard status == noErr else {
            throw HelperError.audioUnitSetDevice(status)
        }
    }

    static func allAudioDeviceIDs() throws -> [AudioDeviceID] {
        var address = AudioObjectPropertyAddress(
            mSelector: kAudioHardwarePropertyDevices,
            mScope: kAudioObjectPropertyScopeGlobal,
            mElement: kAudioObjectPropertyElementMain
        )
        let dataSize = try propertyDataSize(objectID: AudioObjectID(kAudioObjectSystemObject), address: &address)
        let count = Int(dataSize) / MemoryLayout<AudioDeviceID>.size
        var deviceIDs = Array(repeating: AudioDeviceID(0), count: count)
        try deviceIDs.withUnsafeMutableBytes { rawBytes in
            guard let baseAddress = rawBytes.baseAddress else { return }
            var mutableSize = dataSize
            let status = AudioObjectGetPropertyData(
                AudioObjectID(kAudioObjectSystemObject),
                &address,
                0,
                nil,
                &mutableSize,
                baseAddress
            )
            guard status == noErr else {
                throw HelperError.coreAudio("AudioObjectGetPropertyData(devices)", status)
            }
        }
        return deviceIDs
    }

    static func defaultInputDeviceID() throws -> AudioDeviceID {
        var address = AudioObjectPropertyAddress(
            mSelector: kAudioHardwarePropertyDefaultInputDevice,
            mScope: kAudioObjectPropertyScopeGlobal,
            mElement: kAudioObjectPropertyElementMain
        )
        return try propertyValue(objectID: AudioObjectID(kAudioObjectSystemObject), address: &address, as: AudioDeviceID.self)
    }

    static func isInputDevice(_ deviceID: AudioDeviceID) throws -> Bool {
        var address = AudioObjectPropertyAddress(
            mSelector: kAudioDevicePropertyStreamConfiguration,
            mScope: kAudioObjectPropertyScopeInput,
            mElement: kAudioObjectPropertyElementMain
        )
        let dataSize = try propertyDataSize(objectID: deviceID, address: &address)
        let buffer = UnsafeMutableRawPointer.allocate(byteCount: Int(dataSize), alignment: MemoryLayout<AudioBufferList>.alignment)
        defer { buffer.deallocate() }

        var mutableSize = dataSize
        let status = AudioObjectGetPropertyData(deviceID, &address, 0, nil, &mutableSize, buffer)
        guard status == noErr else {
            throw HelperError.coreAudio("AudioObjectGetPropertyData(stream configuration)", status)
        }

        let audioBufferList = buffer.assumingMemoryBound(to: AudioBufferList.self)
        let buffers = UnsafeMutableAudioBufferListPointer(audioBufferList)
        return buffers.contains { $0.mNumberChannels > 0 }
    }

    static func deviceName(_ deviceID: AudioDeviceID) throws -> String {
        var address = AudioObjectPropertyAddress(
            mSelector: kAudioObjectPropertyName,
            mScope: kAudioObjectPropertyScopeGlobal,
            mElement: kAudioObjectPropertyElementMain
        )
        return try stringProperty(objectID: deviceID, address: &address)
    }

    static func deviceUID(_ deviceID: AudioDeviceID) throws -> String {
        var address = AudioObjectPropertyAddress(
            mSelector: kAudioDevicePropertyDeviceUID,
            mScope: kAudioObjectPropertyScopeGlobal,
            mElement: kAudioObjectPropertyElementMain
        )
        return try stringProperty(objectID: deviceID, address: &address)
    }

    static func propertyDataSize(objectID: AudioObjectID, address: inout AudioObjectPropertyAddress) throws -> UInt32 {
        var dataSize: UInt32 = 0
        let status = AudioObjectGetPropertyDataSize(objectID, &address, 0, nil, &dataSize)
        guard status == noErr else {
            throw HelperError.coreAudio("AudioObjectGetPropertyDataSize", status)
        }
        return dataSize
    }

    static func propertyValue<T>(objectID: AudioObjectID, address: inout AudioObjectPropertyAddress, as _: T.Type) throws -> T {
        let pointer = UnsafeMutablePointer<T>.allocate(capacity: 1)
        defer { pointer.deallocate() }

        var dataSize = UInt32(MemoryLayout<T>.stride)
        let status = AudioObjectGetPropertyData(objectID, &address, 0, nil, &dataSize, pointer)
        guard status == noErr else {
            throw HelperError.coreAudio("AudioObjectGetPropertyData", status)
        }
        return pointer.move()
    }

    static func stringProperty(objectID: AudioObjectID, address: inout AudioObjectPropertyAddress) throws -> String {
        let pointer = UnsafeMutablePointer<CFString?>.allocate(capacity: 1)
        pointer.initialize(to: nil)
        defer {
            pointer.deinitialize(count: 1)
            pointer.deallocate()
        }

        var dataSize = UInt32(MemoryLayout<CFString?>.stride)
        let status = AudioObjectGetPropertyData(objectID, &address, 0, nil, &dataSize, pointer)
        guard status == noErr else {
            throw HelperError.coreAudio("AudioObjectGetPropertyData", status)
        }
        guard let value = pointer.pointee else {
            throw HelperError.missingStringProperty("\(address.mSelector)")
        }
        return value as String
    }

    static func convertBuffer(_ buffer: AVAudioPCMBuffer, to targetFormat: AVAudioFormat) throws -> AVAudioPCMBuffer {
        guard buffer.format != targetFormat else {
            return buffer
        }

        guard let converter = AVAudioConverter(from: buffer.format, to: targetFormat) else {
            throw HelperError.noInputFormat
        }
        converter.reset()

        let ratio = targetFormat.sampleRate / buffer.format.sampleRate
        let capacity = AVAudioFrameCount(ceil(Double(buffer.frameLength) * ratio)) + 32
        guard let converted = AVAudioPCMBuffer(pcmFormat: targetFormat, frameCapacity: capacity) else {
            return buffer
        }

        let sourceBuffer = buffer
        final class ConverterState: @unchecked Sendable {
            var consumed = false
        }
        let state = ConverterState()

        var conversionError: NSError?
        let status = converter.convert(to: converted, error: &conversionError) { _, outStatus in
            if state.consumed {
                outStatus.pointee = .endOfStream
                return nil
            }
            state.consumed = true
            outStatus.pointee = .haveData
            return sourceBuffer
        }

        if let conversionError {
            throw conversionError
        }

        switch status {
        case .error:
            throw HelperError.noInputFormat
        case .haveData, .inputRanDry, .endOfStream:
            break
        @unknown default:
            break
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
