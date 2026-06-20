import SwiftUI
import ScreenerKit

/// The popover-style window shown from the menu bar. A header with live status, a
/// section picker (Lists / Suggestions / Settings), and a footer.
struct RootView: View {
    @Environment(AppModel.self) private var model
    @State private var section: Section = .lists

    enum Section: String, CaseIterable, Identifiable {
        case lists = "Lists", suggestions = "Suggestions", settings = "Settings"
        var id: String { rawValue }
    }

    var body: some View {
        VStack(spacing: 0) {
            StatusHeader()
            Divider()

            Picker("", selection: $section) {
                ForEach(Section.allCases) { Text($0.rawValue).tag($0) }
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .padding(8)

            Divider()

            Group {
                switch section {
                case .lists: ListsView()
                case .suggestions: SuggestionsView()
                case .settings: SettingsView()
                }
            }
            .frame(maxHeight: .infinity)

            if let err = model.errorMessage {
                Divider()
                Text(err)
                    .font(.caption)
                    .foregroundStyle(.red)
                    .lineLimit(2)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.horizontal, 10).padding(.vertical, 6)
            }

            Divider()
            Footer()
        }
        .task { await model.refreshAll() }
    }
}

private struct Footer: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        HStack {
            Button {
                Task { await model.refreshAll() }
            } label: {
                Label("Refresh", systemImage: "arrow.clockwise")
            }
            if model.isLoading {
                ProgressView().controlSize(.small).padding(.leading, 4)
            }
            Spacer()
            Button("Quit") { NSApplication.shared.terminate(nil) }
        }
        .padding(.horizontal, 10).padding(.vertical, 6)
    }
}
