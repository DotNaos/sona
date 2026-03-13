// swift-tools-version: 6.2

import PackageDescription

let package = Package(
    name: "sona-apple-stt",
    platforms: [
        .macOS(.v26),
    ],
    products: [
        .executable(name: "sona-apple-stt", targets: ["sona-apple-stt"]),
    ],
    targets: [
        .executableTarget(
            name: "sona-apple-stt"
        ),
    ]
)
