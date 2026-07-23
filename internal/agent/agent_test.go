package agent

import "testing"

func TestExpandAllAndDeduplicates(t *testing.T) {
	got, err := Expand([]Name{Claude, All, Codex})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != Claude || got[1] != Codex {
		t.Fatalf("Expand() = %#v", got)
	}
}
