package lists

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// legacyFiles maps a legacy .txt filename to a list kind. screened_ids.txt is
// intentionally absent (K3 — transient, not a sender list).
var legacyFiles = map[string]Kind{
	"whitelist.txt":      Whitelist,
	"blocklist.txt":      Blocklist,
	"newsletterlist.txt": Newsletter,
	"receiptlist.txt":    Receipts,
}

// legacyGroupAllow are the two hardcoded exceptions from the production
// GROUP_WHITELIST, seeded into the group_allowlist (Spec 0008).
var legacyGroupAllow = []string{
	"drpong@googlegroups.com",
	"haus-k@googlegroups.com",
}

// KindReport is the per-list outcome of a migration.
type KindReport struct {
	Kind    Kind `json:"kind"`
	Read    int  `json:"read"`    // non-blank, non-comment lines
	Kept    int  `json:"kept"`    // newly inserted
	Dupes   int  `json:"dupes"`   // already present
	Junk    int  `json:"junk"`    // failed validation (header leak etc.)
	SelfHit int  `json:"selfHit"` // dropped own address
}

// MigrateReport summarizes a full migration.
type MigrateReport struct {
	Lines []KindReport `json:"lines"`
}

// String renders a one-line-per-list summary.
func (r MigrateReport) String() string {
	var b strings.Builder
	for _, l := range r.Lines {
		fmt.Fprintf(&b, "%-16s read=%-7d kept=%-6d dupes=%-7d junk=%-4d self=%d\n",
			l.Kind, l.Read, l.Kept, l.Dupes, l.Junk, l.SelfHit)
	}
	return b.String()
}

// Migrate imports the legacy .txt lists from dir, cleaning as it goes: dedup
// (via upsert), email/wildcard validation (drops leaked header junk), and drops
// the user's own addresses. It is idempotent. selfAddrs are normalized.
func (s *Store) Migrate(dir string, selfAddrs []string) (*MigrateReport, error) {
	self := map[string]bool{}
	for _, a := range selfAddrs {
		if n := Normalize(a); n != "" {
			self[n] = true
		}
	}

	rep := &MigrateReport{}

	// Stable order for deterministic reports.
	names := make([]string, 0, len(legacyFiles))
	for n := range legacyFiles {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		kind := legacyFiles[name]
		line, err := s.migrateFile(filepath.Join(dir, name), kind, self)
		if err != nil {
			return nil, err
		}
		rep.Lines = append(rep.Lines, line)
	}

	// Seed the legacy group exceptions.
	gr := KindReport{Kind: GroupAllowlist}
	for _, a := range legacyGroupAllow {
		gr.Read++
		ins, err := s.Add(GroupAllowlist, a, "seed")
		if err != nil {
			return nil, err
		}
		if ins {
			gr.Kept++
		} else {
			gr.Dupes++
		}
	}
	rep.Lines = append(rep.Lines, gr)

	return rep, nil
}

func (s *Store) migrateFile(path string, kind Kind, self map[string]bool) (KindReport, error) {
	rep := KindReport{Kind: kind}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rep, nil // missing legacy file is fine
		}
		return rep, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		rep.Read++
		v := Normalize(raw)
		if self[v] {
			rep.SelfHit++
			continue
		}
		if !ValidEntry(v) {
			rep.Junk++
			continue
		}
		ins, err := s.Add(kind, v, "seed")
		if err != nil {
			return rep, err
		}
		if ins {
			rep.Kept++
		} else {
			rep.Dupes++
		}
	}
	return rep, sc.Err()
}
