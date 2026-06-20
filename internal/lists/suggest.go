package lists

import (
	"fmt"
	"sort"
)

// freemailDomains are never collapsed into a "*@domain" wildcard (D5). Includes
// Apple's Hide-My-Mail relay, which would otherwise block every relayed sender.
var freemailDomains = map[string]bool{
	"gmail.com": true, "googlemail.com": true,
	"icloud.com": true, "me.com": true, "mac.com": true,
	"privaterelay.appleid.com": true,
	"outlook.com":              true, "hotmail.com": true, "live.com": true, "msn.com": true,
	"yahoo.com": true, "yahoo.de": true, "ymail.com": true,
	"gmx.de": true, "gmx.net": true, "gmx.com": true,
	"web.de": true, "t-online.de": true, "freenet.de": true,
	"proton.me": true, "protonmail.com": true, "pm.me": true,
	"aol.com": true, "mail.com": true,
}

// IsFreemail reports whether a domain is a free-mail / relay domain.
func IsFreemail(domain string) bool { return freemailDomains[domain] }

// Suggestion proposes collapsing several exact entries into one wildcard.
type Suggestion struct {
	Kind     Kind     `json:"kind"`
	Wildcard string   `json:"wildcard"`
	Covers   []string `json:"covers"`
}

// Suggest mines wildcard compaction candidates across all lists: a non-freemail
// domain with at least min exact entries on a list. It never mutates anything.
func (s *Store) Suggest(min int) ([]Suggestion, error) {
	if min < 2 {
		min = 2
	}
	var out []Suggestion
	for _, k := range AllKinds {
		vals, err := s.All(k)
		if err != nil {
			return nil, err
		}
		byDomain := map[string][]string{}
		for _, v := range vals {
			if IsWildcard(v) {
				continue
			}
			d := Domain(v)
			if d == "" || IsFreemail(d) {
				continue
			}
			byDomain[d] = append(byDomain[d], v)
		}
		for d, covers := range byDomain {
			if len(covers) >= min {
				sort.Strings(covers)
				out = append(out, Suggestion{Kind: k, Wildcard: "*@" + d, Covers: covers})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Wildcard < out[j].Wildcard
	})
	return out, nil
}

// ApplySuggestion inserts the wildcard and removes the exact entries it covers,
// in one transaction. Covered entries are recomputed to stay current.
func (s *Store) ApplySuggestion(k Kind, wildcard string) error {
	if !validKind(k) {
		return fmt.Errorf("unknown list kind %q", k)
	}
	wildcard = Normalize(wildcard)
	if !reWildcard.MatchString(wildcard) {
		return fmt.Errorf("invalid wildcard %q", wildcard)
	}
	domain := Domain(wildcard)

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO entries(list_id, value, is_wildcard, source, created_at)
		VALUES((SELECT id FROM lists WHERE kind=?), ?, 1, 'suggest', ?)`,
		string(k), wildcard, now()); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		DELETE FROM entries
		WHERE list_id=(SELECT id FROM lists WHERE kind=?)
		  AND is_wildcard=0
		  AND value LIKE ?`, string(k), "%@"+domain); err != nil {
		return err
	}
	return tx.Commit()
}
