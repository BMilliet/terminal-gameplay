package tools

import "testing"

func TestBuildListContainsSearchReplaceTool(t *testing.T) {
	got := BuildList()
	if len(got) != 1 {
		t.Fatalf("BuildList() length = %d, want 1", len(got))
	}

	if got[0].T != SearchReplaceAction {
		t.Fatalf("tool title = %q, want %q", got[0].T, SearchReplaceAction)
	}
	if got[0].D == "" {
		t.Fatal("tool description should not be empty")
	}
	if got[0].IsDiv {
		t.Fatal("search/replace should be selectable, got divider")
	}
}
