package classify

// Sender is the single address the engine decides on, already lowercased.
// Address is "" when no address could be parsed upstream.
type Sender struct {
	Address string
}

// Headers carries the deterministic header inputs the engine looks at. Empty
// string means the header was absent.
type Headers struct {
	ListID          string // List-Id
	ListUnsubscribe string // List-Unsubscribe
	ListPost        string // List-Post
	ListHelp        string // List-Help
	XGoogleGroupID  string // X-Google-Group-Id
	Subject         string // Subject (only used when ReceiptSubjectMatch is on)
}

// Matcher answers whether an address belongs to a list, honoring exact match
// first and then "*@domain" wildcards (including parent domains).
type Matcher interface {
	Contains(addr string) bool
}

// Snapshot is an immutable view of the lists the engine consumes. The lists
// package builds it; the engine never reads from disk.
type Snapshot struct {
	Whitelist      Matcher
	Blocklist      Matcher
	Newsletter     Matcher
	Receipts       Matcher
	GroupAllowlist Matcher

	// ReceiptSubjectMatch enables the optional receipt subject-keyword
	// heuristic (Spec 0001 / opt-in, default off). When false the Subject is
	// ignored and classification stays purely list/header-based.
	ReceiptSubjectMatch bool
}

// emptyMatcher is used when a Snapshot field is left nil.
type emptyMatcher struct{}

func (emptyMatcher) Contains(string) bool { return false }

func orEmpty(m Matcher) Matcher {
	if m == nil {
		return emptyMatcher{}
	}
	return m
}
