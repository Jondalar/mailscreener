import SwiftUI
import ScreenerKit

/// Connection dot, version/uptime, and total counts across all lists.
struct StatusHeader: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        let status = model.status
        VStack(alignment: .leading, spacing: 4) {
            HStack(spacing: 6) {
                Circle()
                    .fill(dotColor(status))
                    .frame(width: 9, height: 9)
                Text("ScreenerBar").font(.headline)
                Spacer()
                if let status {
                    Text("v\(status.version)").font(.caption).foregroundStyle(.secondary)
                }
            }

            if let status {
                HStack(spacing: 10) {
                    Label(status.connected ? "Connected" : "Disconnected",
                          systemImage: status.connected ? "wifi" : "wifi.slash")
                    Text("up \(status.uptime)")
                    if let swept = status.lastSweepDate {
                        Text("swept \(swept, format: .relative(presentation: .numeric))")
                    }
                }
                .font(.caption)
                .foregroundStyle(.secondary)

                HStack(spacing: 8) {
                    ForEach(ListKind.allCases) { kind in
                        countChip(kind.displayName, status.size(kind))
                    }
                }
                .padding(.top, 2)
            } else {
                Text(model.settings.isConfigured ? "Loading…" : "Not configured — see Settings.")
                    .font(.caption).foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(10)
    }

    private func dotColor(_ status: Status?) -> Color {
        guard let status else { return .gray }
        return status.connected ? .green : .red
    }

    private func countChip(_ name: String, _ n: Int) -> some View {
        VStack(spacing: 0) {
            Text("\(n)").font(.caption.bold())
            Text(name).font(.system(size: 8)).foregroundStyle(.secondary)
        }
    }
}
