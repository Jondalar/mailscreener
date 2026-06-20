package classify

// Verdict is the classification outcome for one message.
type Verdict string

const (
	Approve    Verdict = "approve"
	Block      Verdict = "block"
	Newsletter Verdict = "newsletter"
	Receipt    Verdict = "receipt"
	Unknown    Verdict = "unknown"
)
