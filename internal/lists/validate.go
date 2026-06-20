package lists

import (
	"regexp"
	"strings"
)

var (
	reEmail    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[a-z]{2,}$`)
	reWildcard = regexp.MustCompile(`^\*@[^@\s]+\.[a-z]{2,}$`)
)

// ValidEntry reports whether v (already normalized) is a usable list value:
// a plausible email address or a "*@domain" wildcard. This is the filter that
// keeps leaked mail-header junk out of the lists (Spec 0002 cleaning).
func ValidEntry(v string) bool {
	return reEmail.MatchString(v) || reWildcard.MatchString(v)
}

// IsWildcard reports whether v is a "*@domain" entry.
func IsWildcard(v string) bool { return strings.HasPrefix(v, "*@") }

// Domain returns the domain part of an address or wildcard, "" if none.
func Domain(v string) string {
	at := strings.LastIndexByte(v, '@')
	if at < 0 || at == len(v)-1 {
		return ""
	}
	return v[at+1:]
}
