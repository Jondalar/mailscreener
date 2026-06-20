import SwiftUI

@main
struct ScreenerBarApp: App {
    @State private var model = AppModel()

    var body: some Scene {
        MenuBarExtra("ScreenerBar", systemImage: "envelope.fill") {
            RootView()
                .environment(model)
                .frame(width: 360, height: 500)
        }
        .menuBarExtraStyle(.window)
    }
}
