package emacs

import "testing"

// Golden-string tests lock the emitted elisp byte-for-byte so any drift in
// quoting, spacing, or argument shape surfaces immediately.

func TestBuildOpenForm_NoFiles(t *testing.T) {
	got := BuildOpenForm("sesh-open-project", "lib", "/abs/cwd", nil)
	want := `(sesh-open-project "lib" "/abs/cwd")`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildOpenForm_RelativeFiles(t *testing.T) {
	got := BuildOpenForm("sesh-open-project", "lib", "/home/me/lib", []string{"README.md", "src/main.go"})
	want := `(sesh-open-project "lib" "/home/me/lib" '("/home/me/lib/README.md" "/home/me/lib/src/main.go"))`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildOpenForm_AbsoluteFiles(t *testing.T) {
	got := BuildOpenForm("sesh-open-project", "lib", "/cwd", []string{"/etc/passwd", "/var/log/foo"})
	want := `(sesh-open-project "lib" "/cwd" '("/etc/passwd" "/var/log/foo"))`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildOpenForm_MixedPaths(t *testing.T) {
	got := BuildOpenForm("sesh-open-project", "lib", "/home/me/lib", []string{"/etc/hosts", "README.md"})
	want := `(sesh-open-project "lib" "/home/me/lib" '("/etc/hosts" "/home/me/lib/README.md"))`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildOpenForm_EscapesQuotesAndBackslashes(t *testing.T) {
	got := BuildOpenForm(
		"sesh-open-project",
		`weird"name`,
		`C:\work\proj`,
		[]string{`/odd"path\file`},
	)
	want := `(sesh-open-project "weird\"name" "C:\\work\\proj" '("/odd\"path\\file"))`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildOpenForm_CustomHookName(t *testing.T) {
	got := BuildOpenForm("my/custom-open", "lib", "/cwd", nil)
	want := `(my/custom-open "lib" "/cwd")`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildCloseForm_DefaultHook(t *testing.T) {
	got := BuildCloseForm("sesh-close-project", "lib")
	want := `(sesh-close-project "lib")`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildCloseForm_CustomHookName(t *testing.T) {
	got := BuildCloseForm("my/custom-close", "lib")
	want := `(my/custom-close "lib")`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestBuildCloseForm_EscapesQuotesAndBackslashes(t *testing.T) {
	got := BuildCloseForm("sesh-close-project", `weird"name\with`)
	want := `(sesh-close-project "weird\"name\\with")`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
