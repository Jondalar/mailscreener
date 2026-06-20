package snooze

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// ParseSnoozedMap reads the legacy snoozed_map.txt (`<folder> <unix_ts>` per
// line) into a folder -> wake-time map. A missing file yields an empty map and
// no error (Spec 0007 best-effort migration).
func ParseSnoozedMap(path string) (map[string]time.Time, error) {
	out := map[string]time.Time{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(strings.TrimSpace(sc.Text()))
		if len(fields) != 2 {
			continue
		}
		ts, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		out[fields[0]] = time.Unix(ts, 0)
	}
	return out, sc.Err()
}

// LabelFromKeyword turns a legacy IMAP keyword flag ("SNOOZED_1W") back into a
// snooze label ("1w"), or "" if the flag is not a snooze keyword.
func LabelFromKeyword(flag string) string {
	const prefix = "SNOOZED_"
	if !strings.HasPrefix(strings.ToUpper(flag), prefix) {
		return ""
	}
	return strings.ToLower(flag[len(prefix):])
}
