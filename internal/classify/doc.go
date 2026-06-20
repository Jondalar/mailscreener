// Package classify is the pure, deterministic classification engine.
//
// It maps a sender plus a snapshot of the lists to a Verdict, with no I/O and
// no side effects, so it is fully unit-testable (ADR-0005).
//
//	Classify(sender, lists) -> Verdict
//
// Rule order (whitelist wins over blocklist, Q4): whitelist -> approve,
// blocklist -> block, newsletter -> newsletter, receipts -> receipt,
// otherwise -> unknown.
package classify
