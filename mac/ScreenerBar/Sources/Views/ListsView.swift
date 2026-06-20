import SwiftUI
import ScreenerKit

/// Pick a list, view/add/remove entries.
struct ListsView: View {
    @Environment(AppModel.self) private var model
    @State private var newValue = ""

    var body: some View {
        @Bindable var model = model
        VStack(spacing: 0) {
            Picker("List", selection: $model.selectedKind) {
                ForEach(ListKind.allCases) { Text($0.displayName).tag($0) }
            }
            .labelsHidden()
            .padding(8)
            .onChange(of: model.selectedKind) {
                Task { await model.loadEntries() }
            }

            List {
                ForEach(model.entries, id: \.self) { value in
                    HStack {
                        Text(value).font(.system(.body, design: .monospaced)).textSelection(.enabled)
                        Spacer()
                        Button(role: .destructive) {
                            Task { await model.remove(value) }
                        } label: {
                            Image(systemName: "trash")
                        }
                        .buttonStyle(.borderless)
                    }
                }
                if model.entries.isEmpty {
                    Text("No entries.").foregroundStyle(.secondary)
                }
            }
            .listStyle(.inset)

            HStack {
                TextField("add address or *@domain", text: $newValue)
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { submit() }
                Button("Add") { submit() }
                    .disabled(newValue.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            .padding(8)
        }
        .task { await model.loadEntries() }
    }

    private func submit() {
        let value = newValue
        newValue = ""
        Task { await model.add(value) }
    }
}
