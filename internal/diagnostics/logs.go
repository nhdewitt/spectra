package diagnostics

import (
	"cmp"
	"encoding/json"
	"slices"
	"unicode/utf8"

	"github.com/nhdewitt/spectra/internal/protocol"
)

// logBudgetBytes is the size FetchLogs trims a result down to.
//
// Deliberately under protocol.MaxCommandResultBytes rather than
// equal to it, since trimToBudget measures the marshalled entries,
// but what actually crosses the wire is the command-result envelope
// wrapped around them, which is what the server applies its limit to.
const logBudgetBytes = protocol.MaxCommandResultBytes * 9 / 10

// truncatedSuffix marks a message shortened by truncateOversized.
const truncatedSuffix = " ... (truncated)"

func dedupKey(source, message string) string {
	return source + "|" + message
}

// levelToPriority maps a level to its syslog priority, where lower is
// more severe.
//
// Every severity comparison goes through here. protocol.LogLevel is a
// string type, so comparing two of them with < or <= compares them
// alphabetically.
func levelToPriority(l protocol.LogLevel) int {
	switch l {
	case protocol.LevelEmergency:
		return 0
	case protocol.LevelAlert:
		return 1
	case protocol.LevelCritical:
		return 2
	case protocol.LevelError:
		return 3
	case protocol.LevelWarning:
		return 4
	case protocol.LevelNotice:
		return 5
	case protocol.LevelInfo:
		return 6
	case protocol.LevelDebug:
		return 7
	default:
		return 6
	}
}

// gatherLimit is how many entries a platform should ask its log source
// for. A request carrying no Limit asks for everything, so the platform
// cap stays the default rather than a fallback that nothing reaches.
func gatherLimit(opts protocol.LogRequest, maxLogs int) int {
	if opts.Limit > 0 && opts.Limit < maxLogs {
		return opts.Limit
	}
	return maxLogs
}

// inWindow reports whether ts falls inside the request's bounds. A zero
// bound is unset, not the epoch.
func inWindow(ts int64, opts protocol.LogRequest) bool {
	if opts.Since > 0 && ts < opts.Since {
		return false
	}
	if opts.Until > 0 && ts > opts.Until {
		return false
	}
	return true
}

// finalize turns a platform's gathered entries into the result that goes
// back to the server windowed, sorted, collapsed, limited, and bounded
// by bytes. Every platform ends its FetchLogs with a call to this, so the
// order those steps run in is decided once.
//
// Collapsing runs before the limit and the byte trim, not after. It is
// the cheapest reduction available and the only one that discards nothing.
// A run of duplicates becomes one entry that still reports how many there
// were. Trimming first would drop entries that collapsing was about to
// make room for.
func finalize(entries []protocol.LogEntry, opts protocol.LogRequest, maxLogs int) []protocol.LogEntry {
	if len(entries) == 0 {
		return []protocol.LogEntry{}
	}

	if opts.Since > 0 || opts.Until > 0 {
		kept := entries[:0]
		for _, e := range entries {
			if inWindow(e.Timestamp, opts) {
				kept = append(kept, e)
			}
		}
		entries = kept
	}

	slices.SortFunc(entries, func(a, b protocol.LogEntry) int {
		return cmp.Compare(a.Timestamp, b.Timestamp)
	})

	entries = collapseDuplicates(entries)

	if limit := gatherLimit(opts, maxLogs); len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	entries = trimToBudget(entries)

	if len(entries) == 0 {
		return []protocol.LogEntry{}
	}

	return entries
}

// collapseDuplicates folds every run of identical source+message
// entries into one, carrying the occurrence count and the timestamp
// of the first. Input and output must be sorted by timestamp.
//
// The surviving entry is the most recent occurrence, not the first.
// Both the byte trim and every platform's own cap discard from the
// old end, so anchoring a collapsed run at its first occurrence would
// put it exactly where the cuts land.
func collapseDuplicates(entries []protocol.LogEntry) []protocol.LogEntry {
	if len(entries) < 2 {
		return entries
	}

	index := make(map[string]int, len(entries))
	out := make([]protocol.LogEntry, 0, len(entries))

	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		key := dedupKey(e.Source, e.Message)

		if pos, ok := index[key]; ok {
			out[pos].Count++
			out[pos].FirstSeen = e.Timestamp
			continue
		}

		e.Count = 1
		e.FirstSeen = e.Timestamp
		index[key] = len(out)
		out = append(out, e)
	}

	for i := range out {
		if out[i].Count < 2 {
			out[i].Count = 0
			out[i].FirstSeen = 0
		}
	}

	slices.Reverse(out)

	return out
}

// trimToBudget drops the oldest entries until the result fits the
// byte budget.
//
// The entry caps each platform applies are a sanity bound on how much
// to gather, not a size limit, so a fixed count cannot bound bytes.
//
// Drops from the front, matching what the platforms already do when
// they exceed MaxLogs.
//
// Marshals rather than estimating. An estimate is what produced the
// mismatch this exists to fix, and the loop converges in one or two
// passes because the overshoot is used to size the cut.
func trimToBudget(entries []protocol.LogEntry) []protocol.LogEntry {
	for len(entries) > 1 {
		encoded, err := json.Marshal(entries)
		if err != nil {
			// Not a size problem. Leave the result alone and let the
			// send path report the error.
			return entries
		}
		if len(encoded) <= logBudgetBytes {
			return entries
		}

		// Size from the overshoot so this converges rather than walking
		// one entry at a time. Round up, and always drop at least one,
		// so an entry larger than the average can't stall the loop.
		perEntry := len(encoded) / len(entries)
		drop := 1
		if perEntry > 0 {
			drop = (len(encoded)-logBudgetBytes)/perEntry + 1
		}
		if drop >= len(entries) {
			drop = len(entries) - 1
		}

		entries = entries[drop:]
	}

	if len(entries) == 1 {
		entries[0] = truncateOversized(entries[0])
	}

	return entries
}

// truncateOversized shortens a lone entry that exceeds the budget by
// itself.
//
// This is the last path that could still produce a 413. trimToBudget
// will not drop to an empty result (an empty log result is
// indistinguishable from a host with nothing to report), so the
// message is cut instead. Cuts land on a rune boundary.
func truncateOversized(e protocol.LogEntry) protocol.LogEntry {
	msg := e.Message

	for {
		encoded, err := json.Marshal([]protocol.LogEntry{e})
		if err != nil || len(encoded) <= logBudgetBytes || msg == "" {
			return e
		}

		cut := len(msg) - (len(encoded) - logBudgetBytes)
		if cut < 0 {
			cut = 0
		}
		for cut > 0 && !utf8.RuneStart(msg[cut]) {
			cut--
		}

		msg = msg[:cut]
		if msg == "" {
			e.Message = ""
		} else {
			e.Message = msg + truncatedSuffix
		}
	}
}
