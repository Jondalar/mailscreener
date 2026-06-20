import Foundation

/// A typed async REST client for the screenerd daemon (Bearer auth).
///
/// `baseURL` must be scheme + host + port only, with no path
/// (e.g. `http://iphnnas:18443`). The client is a value type: rebuild it when the
/// server URL or token changes.
public struct ScreenerClient: Sendable {
    public let baseURL: URL
    public let token: String
    private let session: URLSession

    public init(baseURL: URL, token: String, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
    }

    // MARK: Endpoints

    public func status() async throws -> Status {
        try await get(["status"])
    }

    public func entries(_ kind: ListKind) async throws -> [String] {
        let resp: ListResponse = try await get(["lists", kind.rawValue])
        return resp.entries
    }

    public func add(_ kind: ListKind, value: String) async throws {
        _ = try await send(["lists", kind.rawValue], method: "POST",
                           body: ["value": value], expect: 201)
    }

    public func remove(_ kind: ListKind, value: String) async throws {
        _ = try await send(["lists", kind.rawValue, value], method: "DELETE", expect: 204)
    }

    public func classify(_ req: ClassifyRequest) async throws -> Verdict {
        let resp: VerdictResponse = try await post(["classify"], body: req)
        return resp.verdict
    }

    public func suggestions(min: Int = 5) async throws -> [Suggestion] {
        let resp: SuggestionsResponse = try await get(
            ["suggestions"], query: [URLQueryItem(name: "min", value: String(min))])
        return resp.suggestions
    }

    public func applySuggestion(kind: ListKind, wildcard: String) async throws {
        _ = try await send(["suggestions", "apply"], method: "POST",
                           body: ["kind": kind.rawValue, "wildcard": wildcard], expect: 204)
    }

    // MARK: Plumbing

    private func get<T: Decodable>(_ path: [String], query: [URLQueryItem] = []) async throws -> T {
        let data = try await send(path, method: "GET", query: query, expect: 200)
        return try decode(data)
    }

    private func post<T: Decodable, B: Encodable>(_ path: [String], body: B) async throws -> T {
        let data = try await send(path, method: "POST", body: body, expect: 200)
        return try decode(data)
    }

    @discardableResult
    private func send<B: Encodable>(_ path: [String], method: String,
                                    query: [URLQueryItem] = [], body: B? = nil,
                                    expect: Int) async throws -> Data {
        var req = URLRequest(url: url(path, query: query))
        req.httpMethod = method
        req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        if let body {
            req.setValue("application/json", forHTTPHeaderField: "Content-Type")
            req.httpBody = try JSONEncoder().encode(body)
        }

        let data: Data, response: URLResponse
        do {
            (data, response) = try await session.data(for: req)
        } catch {
            throw ScreenerError.transport(error.localizedDescription)
        }
        guard let http = response as? HTTPURLResponse else {
            throw ScreenerError.transport("no HTTP response")
        }
        if http.statusCode == 401 { throw ScreenerError.unauthorized }
        guard http.statusCode == expect else {
            throw ScreenerError.http(status: http.statusCode, message: errorMessage(data))
        }
        return data
    }

    // Overload for calls without a request body.
    @discardableResult
    private func send(_ path: [String], method: String,
                      query: [URLQueryItem] = [], expect: Int) async throws -> Data {
        try await send(path, method: method, query: query, body: Optional<Empty>.none, expect: expect)
    }

    private func url(_ segments: [String], query: [URLQueryItem]) -> URL {
        var comps = URLComponents()
        comps.scheme = baseURL.scheme
        comps.host = baseURL.host
        comps.port = baseURL.port
        comps.path = "/" + segments.joined(separator: "/")
        if !query.isEmpty { comps.queryItems = query }
        guard let u = comps.url else { fatalError("invalid URL components") }
        return u
    }

    private func decode<T: Decodable>(_ data: Data) throws -> T {
        do {
            return try JSONDecoder().decode(T.self, from: data)
        } catch {
            throw ScreenerError.decoding(error.localizedDescription)
        }
    }

    private func errorMessage(_ data: Data) -> String? {
        struct ErrBody: Decodable { let error: String }
        return try? JSONDecoder().decode(ErrBody.self, from: data).error
    }
}

private struct Empty: Encodable {}
