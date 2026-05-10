package kitty

import "testing"

func TestMatchTabTitleExact(t *testing.T) {
	got := MatchTabTitle("demo:dev")
	want := `tab_title:^demo\:dev$`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMatchTabTitlePrefix(t *testing.T) {
	got := MatchTabTitlePrefix("demo")
	want := `tab_title:^demo\:.*$`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMatchWindowTitleExact(t *testing.T) {
	got := MatchWindowTitle("server")
	want := `title:^server$`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMatchEscapesRegex(t *testing.T) {
	got := MatchTabTitle("a.b+c")
	want := `tab_title:^a\.b\+c$`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProjectTabTitle(t *testing.T) {
	got := ProjectTabTitle("demo", "dev")
	if got != "demo:dev" {
		t.Errorf("got %q, want demo:dev", got)
	}
}
