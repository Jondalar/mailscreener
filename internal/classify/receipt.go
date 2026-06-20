package classify

import "strings"

// receiptSubjectKeywords are the substrings that mark a receipt by subject,
// ported from the production screener.lua RECEIPT_SUBJ list. Matching is
// case-insensitive substring (the subject is lowercased first).
var receiptSubjectKeywords = []string{
	"rechnung", "beleg", "quittung", "steuerbeleg", "zahlungsbeleg",
	"bestellbestätigung", "auftrag", "zahlungseingang", "rechnungsnr", "rechnung nr",
	"invoice", "receipt", "order confirmation", "payment received", "tax invoice",
}

// hasReceiptSubject reports whether a subject contains a receipt keyword.
func hasReceiptSubject(subject string) bool {
	if subject == "" {
		return false
	}
	s := strings.ToLower(subject)
	for _, kw := range receiptSubjectKeywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
