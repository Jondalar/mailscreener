import Foundation

/// The classification outcome, mirroring the daemon's `classify.Verdict`.
public enum Verdict: String, Codable, Sendable {
    case approve, block, newsletter, receipt, unknown
}

/// One of the sender lists, mirroring the daemon's `lists.Kind`. Raw values must
/// match the REST path segments exactly.
public enum ListKind: String, CaseIterable, Codable, Sendable, Identifiable {
    case whitelist
    case blocklist
    case newsletter
    case receipts
    case groupAllowlist = "group_allowlist"

    public var id: String { rawValue }

    public var displayName: String {
        switch self {
        case .whitelist: "Whitelist"
        case .blocklist: "Blocklist"
        case .newsletter: "Newsletter"
        case .receipts: "Receipts"
        case .groupAllowlist: "Group Allowlist"
        }
    }
}

/// `GET /status` response.
public struct Status: Codable, Sendable {
    public let version: String
    public let uptime: String
    public let connected: Bool
    public let lastSweep: String?
    public let lastError: String
    public let listSizes: [String: Int]

    /// `lastSweep` parsed from RFC3339, or nil if never swept.
    public var lastSweepDate: Date? {
        guard let lastSweep else { return nil }
        return ISO8601DateFormatter().date(from: lastSweep)
    }

    public func size(_ kind: ListKind) -> Int { listSizes[kind.rawValue] ?? 0 }
}

/// `GET /lists/{kind}` response.
public struct ListResponse: Codable, Sendable {
    public let kind: String
    public let entries: [String]
}

/// A wildcard-compaction proposal, mirroring the daemon's `lists.Suggestion`.
public struct Suggestion: Codable, Sendable, Identifiable {
    public let kind: ListKind
    public let wildcard: String
    public let covers: [String]

    public var id: String { kind.rawValue + ":" + wildcard }
}

struct SuggestionsResponse: Codable, Sendable {
    let suggestions: [Suggestion]
}

struct VerdictResponse: Codable, Sendable {
    let verdict: Verdict
}

/// `POST /classify` request body. Field names match the server JSON tags.
public struct ClassifyRequest: Codable, Sendable {
    public var sender: String
    public var listId: String
    public var listUnsubscribe: String
    public var listPost: String
    public var listHelp: String
    public var xGoogleGroupId: String

    public init(sender: String, listId: String = "", listUnsubscribe: String = "",
                listPost: String = "", listHelp: String = "", xGoogleGroupId: String = "") {
        self.sender = sender
        self.listId = listId
        self.listUnsubscribe = listUnsubscribe
        self.listPost = listPost
        self.listHelp = listHelp
        self.xGoogleGroupId = xGoogleGroupId
    }
}

/// Errors surfaced by `ScreenerClient`.
public enum ScreenerError: Error, LocalizedError, Sendable {
    case notConfigured
    case unauthorized
    case http(status: Int, message: String?)
    case transport(String)
    case decoding(String)

    public var errorDescription: String? {
        switch self {
        case .notConfigured: "No server URL or token configured."
        case .unauthorized: "Unauthorized — check the API token."
        case let .http(status, message): "HTTP \(status)\(message.map { ": \($0)" } ?? "")"
        case let .transport(m): "Network error: \(m)"
        case let .decoding(m): "Could not read server response: \(m)"
        }
    }
}
