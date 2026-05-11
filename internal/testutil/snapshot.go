package testutil

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var update = flag.Bool("update", false, "update golden files")

// Equal compares got against the contents of goldenPath. Pass -update to
// regenerate the golden file. Use string forms (already-rendered) — for
// structured types, pre-format with fmt.Sprintf or yaml.Marshal.
func Equal(t *testing.T, got string, goldenPath string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", goldenPath, err)
	}
	if diff := cmp.Diff(string(want), got); diff != "" {
		t.Errorf("snapshot mismatch (-want +got):\n%s\n(run with -update if intentional)", diff)
	}
}
