import XCTest
@testable import ScreenerKit

final class DecodingTests: XCTestCase {
    func testStatusDecodes() throws {
        let json = """
        {"version":"nas-20260619","uptime":"3h2m1s","connected":true,
         "lastSweep":"2026-06-19T10:00:00Z","lastError":"",
         "listSizes":{"whitelist":220,"blocklist":1326,"newsletter":303,
                      "receipts":179,"group_allowlist":0}}
        """.data(using: .utf8)!
        let s = try JSONDecoder().decode(Status.self, from: json)
        XCTAssertTrue(s.connected)
        XCTAssertEqual(s.size(.blocklist), 1326)
        XCTAssertNotNil(s.lastSweepDate)
    }

    func testStatusNullSweep() throws {
        let json = """
        {"version":"dev","uptime":"1s","connected":false,"lastSweep":null,
         "lastError":"boom","listSizes":{}}
        """.data(using: .utf8)!
        let s = try JSONDecoder().decode(Status.self, from: json)
        XCTAssertNil(s.lastSweepDate)
        XCTAssertEqual(s.lastError, "boom")
        XCTAssertEqual(s.size(.whitelist), 0)
    }

    func testSuggestionDecodes() throws {
        let json = """
        {"suggestions":[{"kind":"blocklist","wildcard":"*@spam.com",
                         "covers":["a@spam.com","b@spam.com"]}]}
        """.data(using: .utf8)!
        let r = try JSONDecoder().decode(SuggestionsResponse.self, from: json)
        XCTAssertEqual(r.suggestions.first?.kind, .blocklist)
        XCTAssertEqual(r.suggestions.first?.covers.count, 2)
    }

    func testListKindRawValues() {
        XCTAssertEqual(ListKind.groupAllowlist.rawValue, "group_allowlist")
        XCTAssertEqual(ListKind.allCases.count, 5)
    }
}
