// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "ScreenerKit",
    platforms: [.macOS(.v14)],
    products: [
        .library(name: "ScreenerKit", targets: ["ScreenerKit"]),
    ],
    targets: [
        .target(name: "ScreenerKit"),
        .testTarget(name: "ScreenerKitTests", dependencies: ["ScreenerKit"]),
    ]
)
