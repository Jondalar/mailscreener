// Package snooze parses snooze labels into wake times and (in the imap layer)
// drives the Snoozed/<label> folders. The label grammar mirrors the production
// snoozebox.lua (Spec 0007). The parser is pure for table testing.
package snooze

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reNdHour = regexp.MustCompile(`^(\d+)d(\d{1,2})$`)
	reSecs   = regexp.MustCompile(`^(\d+)s$`)
	reHours  = regexp.MustCompile(`^(\d+)h$`)
	reWeekly = regexp.MustCompile(`^([a-z]+)(\d*)$`)
)

var weekdayMap = map[string]time.Weekday{
	"so": time.Sunday, "mo": time.Monday, "di": time.Tuesday, "mi": time.Wednesday,
	"do": time.Thursday, "fr": time.Friday, "sa": time.Saturday,
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseLabel computes the wake time for a snooze label relative to now. It
// returns ok=false for an unknown label. Evaluation order mirrors the legacy
// compute_ts_for_label: "NdHH", "Ns", "Nh", weekday, then fixed labels.
func ParseLabel(label string, now time.Time) (time.Time, bool) {
	label = strings.ToLower(strings.TrimSpace(label))
	loc := now.Location()

	atHour := func(base time.Time, h int) time.Time {
		return time.Date(base.Year(), base.Month(), base.Day(), h, 0, 0, 0, loc)
	}

	// "NdHH": in N days at hour HH.
	if m := reNdHour.FindStringSubmatch(label); m != nil {
		d, _ := strconv.Atoi(m[1])
		h := clampHour(atoi(m[2]))
		ts := atHour(now.AddDate(0, 0, d), h)
		if d == 0 && !ts.After(now) {
			ts = ts.AddDate(0, 0, 1)
		}
		return ts, true
	}

	// "Ns": now + N seconds.
	if m := reSecs.FindStringSubmatch(label); m != nil {
		return now.Add(time.Duration(atoi(m[1])) * time.Second), true
	}

	// "Nh": now + N hours.
	if m := reHours.FindStringSubmatch(label); m != nil {
		return now.Add(time.Duration(atoi(m[1])) * time.Hour), true
	}

	// Weekday name + optional hour (default 08:00): next occurrence.
	if m := reWeekly.FindStringSubmatch(label); m != nil {
		if target, ok := weekdayMap[m[1]]; ok {
			h := 8
			if m[2] != "" {
				h = clampHour(atoi(m[2]))
			}
			return nextWeekday(now, target, h), true
		}
	}

	// Fixed labels.
	switch label {
	case "1d10", "1d":
		return atHour(now.AddDate(0, 0, 1), 10), true
	case "1w":
		return atHour(now.AddDate(0, 0, 7), 10), true
	case "2w":
		return atHour(now.AddDate(0, 0, 14), 10), true
	case "1m":
		return atHour(addMonths(now, 1), 10), true
	case "3m":
		return atHour(addMonths(now, 3), 10), true
	}

	return time.Time{}, false
}

func nextWeekday(now time.Time, target time.Weekday, hour int) time.Time {
	days := (int(target) - int(now.Weekday()) + 7) % 7
	if days == 0 {
		days = 7 // always the next occurrence, never today
	}
	base := now.AddDate(0, 0, days)
	return time.Date(base.Year(), base.Month(), base.Day(), hour, 0, 0, 0, now.Location())
}

// addMonths adds n calendar months, clamping the day to the target month's
// length (Jan 31 + 1m -> Feb 28/29), matching the legacy behavior.
func addMonths(t time.Time, n int) time.Time {
	total := int(t.Month()) - 1 + n
	y := t.Year() + total/12
	m := time.Month(total%12 + 1)
	d := t.Day()
	if dim := daysInMonth(y, m); d > dim {
		d = dim
	}
	return time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), 0, t.Location())
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func clampHour(h int) int {
	if h < 0 {
		return 0
	}
	if h > 23 {
		return 23
	}
	return h
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }
