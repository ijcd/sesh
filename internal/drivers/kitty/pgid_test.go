package kitty

import (
	"context"
	"reflect"
	"testing"
)

// alwaysLeader is a pgidLookup that returns each queried pid as its own
// group leader. Useful in fixture-based tests where the simulated process
// is implicitly the user-typed command (the most common case).
func alwaysLeader(_ context.Context, pids []int) map[int]int {
	out := make(map[int]int, len(pids))
	for _, p := range pids {
		out[p] = p
	}
	return out
}

func TestParsePsOutput_HappyPath(t *testing.T) {
	in := "  1234  1234\n  1235  1234\n  1240  1240\n"
	got := parsePsOutput(in)
	want := map[int]int{1234: 1234, 1235: 1234, 1240: 1240}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParsePsOutput_IgnoresMalformed(t *testing.T) {
	in := "  1234  1234\nbogus line\n  garbage  garbage\n  1240  1240\n"
	got := parsePsOutput(in)
	want := map[int]int{1234: 1234, 1240: 1240}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParsePsOutput_Empty(t *testing.T) {
	got := parsePsOutput("")
	if len(got) != 0 {
		t.Errorf("got %v, want empty map", got)
	}
}

func TestPickForegroundCmd_PicksGroupLeader(t *testing.T) {
	// claude (pid 100, pgid 100) is the group leader.
	// caffeinate (pid 101, pgid 100) is its child.
	procs := []kittyForegroundProc{
		{Pid: 101, Cmdline: []string{"caffeinate", "-i", "-t", "300"}},
		{Pid: 100, Cmdline: []string{"claude", "--continue"}},
	}
	pgids := map[int]int{100: 100, 101: 100}
	got := pickForegroundCmd(procs, pgids)
	want := []string{"claude", "--continue"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPickForegroundCmd_FiltersSelfCapture(t *testing.T) {
	// The kitten ls process is filtered out before group-leader selection.
	procs := []kittyForegroundProc{
		{Pid: 200, Cmdline: []string{"/opt/homebrew/bin/kitten", "@", "--to", "unix:/tmp/kitty-3063", "ls"}},
		{Pid: 100, Cmdline: []string{"claude", "--continue"}},
	}
	pgids := map[int]int{100: 100, 200: 200}
	got := pickForegroundCmd(procs, pgids)
	want := []string{"claude", "--continue"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPickForegroundCmd_FallbackPicksLowestPid(t *testing.T) {
	// No process has pid == pgid (leader isn't in the list) → fallback path.
	// Option B: return the cmdline of the lowest-PID process.
	procs := []kittyForegroundProc{
		{Pid: 102, Cmdline: []string{"helper", "two"}},
		{Pid: 101, Cmdline: []string{"helper", "one"}},
	}
	pgids := map[int]int{101: 100, 102: 100}
	got := pickForegroundCmd(procs, pgids)
	want := []string{"helper", "one"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected lowest-PID cmdline %v, got %v", want, got)
	}
}

func TestPickForegroundCmd_FallbackIgnoresZeroPid(t *testing.T) {
	// Pid 0 means "info missing" — should be ignored when picking lowest.
	procs := []kittyForegroundProc{
		{Pid: 0, Cmdline: []string{"missing", "pid"}},
		{Pid: 105, Cmdline: []string{"real", "process"}},
	}
	got := pickForegroundCmd(procs, map[int]int{105: 200})
	want := []string{"real", "process"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected real-pid cmdline %v, got %v", want, got)
	}
}

func TestPickForegroundCmd_FallbackAllZeroPidsReturnsFirst(t *testing.T) {
	// If every process has pid 0 (no info at all), fall back to procs[0].
	procs := []kittyForegroundProc{
		{Pid: 0, Cmdline: []string{"first"}},
		{Pid: 0, Cmdline: []string{"second"}},
	}
	got := pickForegroundCmd(procs, nil)
	want := []string{"first"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected first cmdline %v, got %v", want, got)
	}
}

func TestPickForegroundCmd_NoProcsAfterFilter(t *testing.T) {
	// Only the kitten ls process is in the list; after filtering it's empty.
	procs := []kittyForegroundProc{
		{Pid: 200, Cmdline: []string{"kitten", "ls"}},
	}
	got := pickForegroundCmd(procs, map[int]int{200: 200})
	if got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestIsKittenLs(t *testing.T) {
	cases := []struct {
		in   []string
		want bool
	}{
		{[]string{"/opt/homebrew/bin/kitten", "@", "--to", "unix:/tmp/sock", "ls"}, true},
		{[]string{"kitten", "ls"}, true},
		{[]string{"/Applications/kitty.app/Contents/MacOS/kitten", "ls"}, true},
		{[]string{"kitten", "@", "launch", "--type=tab"}, false}, // kitten but not ls
		{[]string{"claude", "ls"}, false},                        // ls but not kitten
		{[]string{}, false},
	}
	for _, c := range cases {
		got := isKittenLs(c.in)
		if got != c.want {
			t.Errorf("isKittenLs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestJoinInts(t *testing.T) {
	if got := joinInts(nil, ","); got != "" {
		t.Errorf("nil → %q", got)
	}
	if got := joinInts([]int{1234}, ","); got != "1234" {
		t.Errorf("single → %q", got)
	}
	if got := joinInts([]int{1234, 5678, 90}, ","); got != "1234,5678,90" {
		t.Errorf("multi → %q", got)
	}
}
