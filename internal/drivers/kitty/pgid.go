package kitty

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// pgidLookup maps pids to their process group ids. Used as a seam so tests
// can inject fake data without shelling out to `ps`.
type pgidLookup func(ctx context.Context, pids []int) map[int]int

// systemPgidLookup is the production implementation: shells out to
// `ps -o pid=,pgid= -p <pids>` and parses the result. Single subprocess
// per capture run (not per process).
func systemPgidLookup(ctx context.Context, pids []int) map[int]int {
	if len(pids) == 0 {
		return nil
	}
	args := []string{"-o", "pid=,pgid=", "-p", joinInts(pids, ",")}
	out, err := exec.CommandContext(ctx, "ps", args...).Output()
	if err != nil {
		return nil
	}
	return parsePsOutput(string(out))
}

// parsePsOutput converts `ps -o pid=,pgid= ...` output to a pid→pgid map.
// Pure function — testable without spawning a subprocess.
//
// Example input:
//
//	1234   1234
//	1235   1234
//	1240   1240
//
// Returns: {1234: 1234, 1235: 1234, 1240: 1240}.
func parsePsOutput(out string) map[int]int {
	result := map[int]int{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		pgid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		result[pid] = pgid
	}
	return result
}

// pickForegroundCmd selects the cmdline of the user-typed foreground command
// from a list of foreground processes. Steps:
//  1. Filter out self-capture (the running `kitten ls` invocation).
//  2. Among the remainder, pick the process where pid == pgid — the
//     POSIX process-group leader. This is what the kernel calls "the
//     foreground command."
//  3. If no group leader is identifiable in the list, defer to fallbackCmd.
//
// Returns nil if nothing survives filtering.
func pickForegroundCmd(procs []kittyForegroundProc, pgids map[int]int) []string {
	procs = filterSelfCapture(procs)
	if len(procs) == 0 {
		return nil
	}
	for _, p := range procs {
		if pgid, ok := pgids[p.Pid]; ok && pgid == p.Pid {
			return p.Cmdline
		}
	}
	return fallbackCmd(procs)
}

// filterSelfCapture removes processes that look like the sesh-spawned
// `kitten ls` invocation we just ran. Otherwise the tab from which sesh
// was launched would show that command as its current process.
func filterSelfCapture(procs []kittyForegroundProc) []kittyForegroundProc {
	out := procs[:0:0]
	for _, p := range procs {
		if isKittenLs(p.Cmdline) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// isKittenLs reports whether cmdline is a `kitten @ ... ls` or `kitten ls`
// invocation. Detected by the binary basename being "kitten" and "ls" appearing
// as a discrete arg.
func isKittenLs(cmdline []string) bool {
	if len(cmdline) == 0 {
		return false
	}
	if lastPathSegment(cmdline[0]) != "kitten" {
		return false
	}
	for _, a := range cmdline[1:] {
		if a == "ls" {
			return true
		}
	}
	return false
}

// fallbackCmd is called when pickForegroundCmd cannot identify a process
// group leader (no process in the list has pid == its own pgid). This happens
// when `ps` is unavailable (containers, restricted sandboxes), when pgid info
// is missing for these pids, or in edge cases like wrapper scripts that
// exec'd into a different binary mid-flight.
//
// Strategy: return the cmdline of the LOWEST-PID process.
//
// Why: PIDs are assigned monotonically as processes are created. When a user
// types `claude` and claude later forks `caffeinate`, claude has the lower
// PID — it existed first. The earliest-started process in a tab's foreground
// group is almost always the user-typed command; later-started processes are
// helpers it spawned. This is a heuristic, not a guarantee — it's wrong for
// wrapper scripts that exec into a different binary (the exec inherits the
// original PID), but those cases are rare. The honest "return nil" option
// would force the user to edit every uncertain capture; this guess is right
// in the common case and only loses to the pgid path when pgid is unknown.
//
// Returns nil only when procs is empty (filtered to nothing).
func fallbackCmd(procs []kittyForegroundProc) []string {
	if len(procs) == 0 {
		return nil
	}
	best := procs[0]
	for _, p := range procs[1:] {
		// Skip pid=0 (missing pid info — JSON didn't include the field).
		// If best.Pid is also 0, any real pid beats it; otherwise prefer
		// the strictly lower real pid.
		if p.Pid <= 0 {
			continue
		}
		if best.Pid <= 0 || p.Pid < best.Pid {
			best = p
		}
	}
	return best.Cmdline
}

func joinInts(xs []int, sep string) string {
	if len(xs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, x := range xs {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(strconv.Itoa(x))
	}
	return b.String()
}
