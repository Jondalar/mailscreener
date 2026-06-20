package classify

// Classify decides the verdict for one message. It is pure and deterministic
// (ADR-0005): same inputs always yield the same verdict, no I/O.
//
// Rule order (first match wins):
//  1. whitelist        -> approve   (always wins, Q4/QS3)
//  2. group filter     -> block     (group message not on group_allowlist, Spec 0008)
//  3. blocklist        -> block
//  4. newsletter       -> newsletter (list OR List-Id/List-Unsubscribe header)
//  5. receipts         -> receipt   (list only, no subject heuristics — D4)
//  6. otherwise        -> unknown   (stays in Screened)
func Classify(s Sender, h Headers, lists Snapshot) Verdict {
	wl := orEmpty(lists.Whitelist)
	bl := orEmpty(lists.Blocklist)
	nl := orEmpty(lists.Newsletter)
	rc := orEmpty(lists.Receipts)
	ga := orEmpty(lists.GroupAllowlist)

	addr := s.Address

	// 1. Whitelist beats everything (a wanted sender is never junked).
	if addr != "" && wl.Contains(addr) {
		return Approve
	}

	// 2. Group filter: group-looking mail that is not allowlisted -> block.
	// Allowlisted group mail falls through and is typically caught as a
	// newsletter via its List-Id header below.
	if IsGroupMessage(h) && !(addr != "" && ga.Contains(addr)) {
		return Block
	}

	// 3. Blocklist.
	if addr != "" && bl.Contains(addr) {
		return Block
	}

	// 4. Newsletter: explicit list, or RFC list headers.
	if (addr != "" && nl.Contains(addr)) || h.ListID != "" || h.ListUnsubscribe != "" {
		return Newsletter
	}

	// 5. Receipts: list, plus the optional subject-keyword heuristic (opt-in).
	if addr != "" && rc.Contains(addr) {
		return Receipt
	}
	if lists.ReceiptSubjectMatch && hasReceiptSubject(h.Subject) {
		return Receipt
	}

	return Unknown
}
