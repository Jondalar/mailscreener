package classify

import "testing"

func snap(wl, bl, nl, rc, ga []string) Snapshot {
	return Snapshot{
		Whitelist:      NewMapMatcher(wl),
		Blocklist:      NewMapMatcher(bl),
		Newsletter:     NewMapMatcher(nl),
		Receipts:       NewMapMatcher(rc),
		GroupAllowlist: NewMapMatcher(ga),
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		s    Sender
		h    Headers
		l    Snapshot
		want Verdict
	}{
		{
			name: "whitelist wins over blocklist (QS3)",
			s:    Sender{"a@b.com"},
			l:    snap([]string{"a@b.com"}, []string{"a@b.com"}, nil, nil, nil),
			want: Approve,
		},
		{
			name: "whitelist wins over List-Id newsletter heuristic",
			s:    Sender{"a@b.com"},
			h:    Headers{ListID: "<list.b.com>"},
			l:    snap([]string{"a@b.com"}, nil, nil, nil, nil),
			want: Approve,
		},
		{
			name: "blocklist blocks",
			s:    Sender{"spam@x.com"},
			l:    snap(nil, []string{"spam@x.com"}, nil, nil, nil),
			want: Block,
		},
		{
			name: "receipts list only",
			s:    Sender{"shop@store.com"},
			l:    snap(nil, nil, nil, []string{"shop@store.com"}, nil),
			want: Receipt,
		},
		{
			name: "receipts sender also whitelisted -> approve",
			s:    Sender{"shop@store.com"},
			l:    snap([]string{"shop@store.com"}, nil, nil, []string{"shop@store.com"}, nil),
			want: Approve,
		},
		{
			name: "newsletter via list",
			s:    Sender{"news@m.com"},
			l:    snap(nil, nil, []string{"news@m.com"}, nil, nil),
			want: Newsletter,
		},
		{
			name: "newsletter via List-Unsubscribe header",
			s:    Sender{"x@y.com"},
			h:    Headers{ListUnsubscribe: "<mailto:u@y.com>"},
			l:    snap(nil, nil, nil, nil, nil),
			want: Newsletter,
		},
		{
			name: "wildcard matches exact domain",
			s:    Sender{"x@domain.com"},
			l:    snap([]string{"*@domain.com"}, nil, nil, nil, nil),
			want: Approve,
		},
		{
			name: "wildcard matches subdomain",
			s:    Sender{"x@sub.domain.com"},
			l:    snap([]string{"*@domain.com"}, nil, nil, nil, nil),
			want: Approve,
		},
		{
			name: "wildcard does not match unrelated domain",
			s:    Sender{"x@notdomain.com"},
			l:    snap([]string{"*@domain.com"}, nil, nil, nil, nil),
			want: Unknown,
		},
		{
			name: "exact whitelist beats conflicting blocklist wildcard",
			s:    Sender{"a@d.com"},
			l:    snap([]string{"a@d.com"}, []string{"*@d.com"}, nil, nil, nil),
			want: Approve,
		},
		{
			name: "group message not allowlisted -> block",
			s:    Sender{"g@googlegroups.com"},
			h:    Headers{XGoogleGroupID: "abc"},
			l:    snap(nil, nil, nil, nil, nil),
			want: Block,
		},
		{
			name: "group message via List-Id + List-Post -> block",
			s:    Sender{"g@lists.example.com"},
			h:    Headers{ListID: "<l.example.com>", ListPost: "<mailto:l@example.com>"},
			l:    snap(nil, nil, nil, nil, nil),
			want: Block,
		},
		{
			name: "allowlisted group with List-Id -> newsletter (not blocked)",
			s:    Sender{"drpong@googlegroups.com"},
			h:    Headers{XGoogleGroupID: "abc", ListID: "<drpong.googlegroups.com>"},
			l:    snap(nil, nil, nil, nil, []string{"drpong@googlegroups.com"}),
			want: Newsletter,
		},
		{
			name: "whitelisted group -> approve",
			s:    Sender{"drpong@googlegroups.com"},
			h:    Headers{XGoogleGroupID: "abc"},
			l:    snap([]string{"drpong@googlegroups.com"}, nil, nil, nil, nil),
			want: Approve,
		},
		{
			name: "List-Id alone is not a group -> newsletter",
			s:    Sender{"x@y.com"},
			h:    Headers{ListID: "<l.y.com>"},
			l:    snap(nil, nil, nil, nil, nil),
			want: Newsletter,
		},
		{
			name: "empty sender, no headers -> unknown",
			s:    Sender{""},
			l:    snap(nil, nil, nil, nil, nil),
			want: Unknown,
		},
		{
			name: "receipt subject ignored when heuristic off",
			s:    Sender{"x@y.com"},
			h:    Headers{Subject: "Ihre Rechnung 123"},
			l:    snap(nil, nil, nil, nil, nil),
			want: Unknown,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.s, c.h, c.l); got != c.want {
				t.Errorf("Classify() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestReceiptSubjectHeuristic(t *testing.T) {
	s := snap(nil, nil, nil, nil, nil)
	s.ReceiptSubjectMatch = true

	if got := Classify(Sender{"x@y.com"}, Headers{Subject: "Ihre Rechnung Nr. 42"}, s); got != Receipt {
		t.Errorf("invoice subject -> %q, want receipt", got)
	}
	if got := Classify(Sender{"x@y.com"}, Headers{Subject: "hello there"}, s); got != Unknown {
		t.Errorf("plain subject -> %q, want unknown", got)
	}
	// Whitelist still wins over the subject heuristic.
	sw := snap([]string{"x@y.com"}, nil, nil, nil, nil)
	sw.ReceiptSubjectMatch = true
	if got := Classify(Sender{"x@y.com"}, Headers{Subject: "invoice"}, sw); got != Approve {
		t.Errorf("whitelisted invoice -> %q, want approve", got)
	}
}
