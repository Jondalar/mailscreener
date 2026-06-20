import SwiftUI
import ScreenerKit

/// Server URL + API token, with a connection test. Token is stored in the Keychain.
struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @State private var testResult: String?

    var body: some View {
        @Bindable var settings = model.settings
        Form {
            Section("Server") {
                TextField("Base URL", text: $settings.baseURLString, prompt: Text("http://iphnnas:18443"))
                    .textFieldStyle(.roundedBorder)
                    .autocorrectionDisabled()
                SecureField("API Token", text: $settings.token)
                    .textFieldStyle(.roundedBorder)
            }

            Section {
                HStack {
                    Button("Test Connection") { Task { await test() } }
                        .disabled(!settings.isConfigured)
                    if let testResult {
                        Text(testResult).font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
        }
        .formStyle(.grouped)
    }

    private func test() async {
        testResult = "Testing…"
        guard let client = model.settings.client else {
            testResult = "Not configured."
            return
        }
        do {
            let status = try await client.status()
            testResult = "OK — \(status.connected ? "daemon connected" : "daemon offline"), v\(status.version)"
            model.status = status
        } catch {
            testResult = (error as? ScreenerError)?.localizedDescription ?? error.localizedDescription
        }
    }
}
