package tools

import "testing"

func TestBuildListContainsFileSearchAndSearchReplaceTools(t *testing.T) {
	got := BuildList()
	if len(got) != 3 {
		t.Fatalf("BuildList() length = %d, want 3", len(got))
	}

	want := []string{FuzzySearchAction, WorkspaceFuzzySearchAction, SearchReplaceAction}
	for index, title := range want {
		if got[index].T != title {
			t.Fatalf("tool %d title = %q, want %q", index, got[index].T, title)
		}
		if got[index].D == "" {
			t.Fatalf("tool %q description should not be empty", title)
		}
		if got[index].IsDiv {
			t.Fatalf("tool %q should be selectable, got divider", title)
		}
	}
}
