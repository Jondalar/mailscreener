import SwiftUI
import ScreenerKit

/// Wildcard-compaction proposals from the daemon, each applyable with one tap.
struct SuggestionsView: View {
    @Environment(AppModel.self) private var model
    @State private var minCount = 5

    var body: some View {
        VStack(spacing: 0) {
            Stepper("Min entries: \(minCount)", value: $minCount, in: 2...50)
                .padding(8)
                .onChange(of: minCount) {
                    Task { await model.loadSuggestions(min: minCount) }
                }

            List {
                ForEach(model.suggestions) { s in
                    VStack(alignment: .leading, spacing: 2) {
                        HStack {
                            Text(s.wildcard).font(.system(.body, design: .monospaced))
                            Spacer()
                            Text(s.kind.displayName).font(.caption).foregroundStyle(.secondary)
                            Button("Apply") { Task { await model.apply(s) } }
                        }
                        Text("collapses \(s.covers.count) entries")
                            .font(.caption).foregroundStyle(.secondary)
                    }
                }
                if model.suggestions.isEmpty {
                    Text("No suggestions.").foregroundStyle(.secondary)
                }
            }
            .listStyle(.inset)
        }
        .task { await model.loadSuggestions(min: minCount) }
    }
}
