import Foundation
import Observation
import ScreenerKit

/// The view model behind the menu-bar UI. Owns settings, the loaded data, and the
/// async actions that talk to the daemon. All mutation happens on the main actor.
@MainActor
@Observable
final class AppModel {
    let settings = AppSettings()

    var status: Status?
    var selectedKind: ListKind = .whitelist
    var entries: [String] = []
    var suggestions: [Suggestion] = []

    var isLoading = false
    var errorMessage: String?

    // MARK: Status

    func refreshStatus() async {
        await run { client in self.status = try await client.status() }
    }

    // MARK: Lists

    func loadEntries() async {
        let kind = selectedKind
        await run { client in self.entries = try await client.entries(kind) }
    }

    func add(_ value: String) async {
        let trimmed = value.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty else { return }
        let kind = selectedKind
        await run { client in
            try await client.add(kind, value: trimmed)
            self.entries = try await client.entries(kind)
        }
    }

    func remove(_ value: String) async {
        let kind = selectedKind
        await run { client in
            try await client.remove(kind, value: value)
            self.entries.removeAll { $0 == value }
        }
    }

    // MARK: Suggestions

    func loadSuggestions(min: Int = 5) async {
        await run { client in self.suggestions = try await client.suggestions(min: min) }
    }

    func apply(_ suggestion: Suggestion) async {
        await run { client in
            try await client.applySuggestion(kind: suggestion.kind, wildcard: suggestion.wildcard)
            self.suggestions.removeAll { $0.id == suggestion.id }
        }
    }

    /// Refresh everything currently on screen.
    func refreshAll() async {
        await refreshStatus()
        await loadEntries()
    }

    // MARK: Helper

    /// Runs an action against the configured client, funnelling errors into
    /// `errorMessage` and toggling `isLoading`.
    private func run(_ action: (ScreenerClient) async throws -> Void) async {
        guard let client = settings.client else {
            errorMessage = ScreenerError.notConfigured.localizedDescription
            return
        }
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }
        do {
            try await action(client)
        } catch {
            errorMessage = (error as? ScreenerError)?.localizedDescription ?? error.localizedDescription
        }
    }
}
