import Foundation
import Observation
import ScreenerKit

/// User-editable connection settings. The base URL lives in UserDefaults; the
/// token is kept in the Keychain. Exposes a ready-to-use `ScreenerClient` when
/// both are valid.
@MainActor
@Observable
final class AppSettings {
    /// e.g. "http://iphnnas:18443"
    var baseURLString: String {
        didSet { UserDefaults.standard.set(baseURLString, forKey: Self.urlKey) }
    }

    /// Backed by the Keychain, not UserDefaults.
    var token: String {
        didSet {
            if token.isEmpty { Keychain.delete() } else { Keychain.set(token) }
        }
    }

    private static let urlKey = "screener.baseURL"

    init() {
        baseURLString = UserDefaults.standard.string(forKey: Self.urlKey) ?? "http://iphnnas:18443"
        token = Keychain.get() ?? ""
    }

    var isConfigured: Bool { client != nil }

    /// A client built from the current settings, or nil if incomplete/invalid.
    var client: ScreenerClient? {
        let trimmed = baseURLString.trimmingCharacters(in: .whitespaces)
        guard !trimmed.isEmpty, !token.isEmpty,
              let url = URL(string: trimmed), url.host != nil else { return nil }
        return ScreenerClient(baseURL: url, token: token)
    }
}
