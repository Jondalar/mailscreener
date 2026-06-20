package classify

import "strings"

// MapMatcher matches an address against a set of entries. Entries are either an
// exact lowercased address ("a@b.com") or a wildcard ("*@domain.com"). A
// wildcard matches the exact domain and any subdomain of it.
type MapMatcher struct {
	entries map[string]struct{}
}

// NewMapMatcher builds a matcher from lowercased entries. Values are used as-is;
// callers are expected to have trimmed and lowercased them.
func NewMapMatcher(values []string) *MapMatcher {
	m := &MapMatcher{entries: make(map[string]struct{}, len(values))}
	for _, v := range values {
		if v != "" {
			m.entries[v] = struct{}{}
		}
	}
	return m
}

// Contains reports whether addr is covered by an exact entry or a wildcard.
func (m *MapMatcher) Contains(addr string) bool {
	if m == nil || addr == "" {
		return false
	}
	if _, ok := m.entries[addr]; ok {
		return true
	}
	at := strings.LastIndexByte(addr, '@')
	if at < 0 || at == len(addr)-1 {
		return false
	}
	domain := addr[at+1:]
	// Check the domain and each parent suffix down to (but not including) the
	// final single-label TLD: "*@a.b.com" and "*@b.com" both cover "x@a.b.com".
	for d := domain; strings.IndexByte(d, '.') >= 0; {
		if _, ok := m.entries["*@"+d]; ok {
			return true
		}
		dot := strings.IndexByte(d, '.')
		d = d[dot+1:]
	}
	return false
}
