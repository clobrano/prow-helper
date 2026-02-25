package selector

import (
	"fmt"
	"testing"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query  string
		target string
		want   bool
	}{
		{"", "anything", true},
		{"ovn", "pull-ci-openshift-e2e-aws-ovn", true},
		{"aws", "pull-ci-openshift-e2e-aws-ovn", true},
		{"gcp", "pull-ci-openshift-e2e-aws-ovn", false},
		{"OVN", "pull-ci-openshift-e2e-aws-ovn", true},  // case-insensitive
		{"aon", "pull-ci-openshift-e2e-aws-ovn", false}, // not a contiguous substring
		{"noa", "pull-ci-openshift-e2e-aws-ovn", false},
		{"x", "pull-ci-openshift-e2e-aws-ovn", false},
		{"pending", "[ 1] pending  some-job-name", true},
		{"pending", "[ 5] failure  some-job-fencing", false}, // must not match "fencing"
	}
	for _, tt := range tests {
		got := fuzzyMatch(tt.query, tt.target)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.query, tt.target, got, tt.want)
		}
	}
}

func TestRefilter(t *testing.T) {
	items := []Item{
		{Label: "success  pull-ci-aws-ovn  111"},
		{Label: "failure  pull-ci-gcp-sdn  222"},
		{Label: "pending  pull-ci-aws-sdn  333"},
	}
	m := newModel(items, nil)

	// No query — all items visible.
	if len(m.filtered) != 3 {
		t.Fatalf("expected 3 filtered, got %d", len(m.filtered))
	}

	m.query = "aws"
	m.refilter()
	if len(m.filtered) != 2 {
		t.Fatalf("query 'aws': expected 2 filtered, got %d", len(m.filtered))
	}

	m.query = "gcp"
	m.refilter()
	if len(m.filtered) != 1 {
		t.Fatalf("query 'gcp': expected 1 filtered, got %d", len(m.filtered))
	}

	m.query = "zzz"
	m.refilter()
	if len(m.filtered) != 0 {
		t.Fatalf("query 'zzz': expected 0 filtered, got %d", len(m.filtered))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should be 0 when filtered is empty, got %d", m.cursor)
	}
}

func TestToggleAll(t *testing.T) {
	items := []Item{{Label: "a"}, {Label: "b"}, {Label: "c"}}
	m := newModel(items, nil)

	// First A: select all.
	m, _ = toggleAll(m)
	for _, fi := range m.filtered {
		if !m.selected[fi] {
			t.Errorf("item %d should be selected after first A", fi)
		}
	}

	// Second A: deselect all.
	m, _ = toggleAll(m)
	for _, fi := range m.filtered {
		if m.selected[fi] {
			t.Errorf("item %d should be deselected after second A", fi)
		}
	}
}

// toggleAll simulates pressing Ctrl+A by calling the same logic used in Update.
func toggleAll(m model) (model, bool) {
	allSelected := true
	for _, fi := range m.filtered {
		if !m.selected[fi] {
			allSelected = false
			break
		}
	}
	for _, fi := range m.filtered {
		m.selected[fi] = !allSelected
	}
	return m, allSelected
}

func TestCursorBoundaries(t *testing.T) {
	items := []Item{{Label: "a"}, {Label: "b"}, {Label: "c"}}
	m := newModel(items, nil)
	m.cursor = 2

	// Narrow the query so only one item is visible; cursor must clamp.
	m.query = "a"
	m.refilter()
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}

func TestVisibleLines(t *testing.T) {
	items := []Item{{Label: "a"}, {Label: "b"}, {Label: "c"}}
	m := newModel(items, nil)

	// Height not set yet — show all filtered items.
	if got := m.visibleLines(); got != 3 {
		t.Errorf("visibleLines with height=0: got %d, want 3", got)
	}

	// Height smaller than overhead — at least 1 item.
	m.height = 2
	if got := m.visibleLines(); got != 1 {
		t.Errorf("visibleLines with height=2 (< overhead): got %d, want 1", got)
	}

	// Normal height: height - overhead rows for items.
	m.height = 20
	if got := m.visibleLines(); got != 20-viewOverhead {
		t.Errorf("visibleLines with height=20: got %d, want %d", got, 20-viewOverhead)
	}
}

func TestViewportStart(t *testing.T) {
	items := make([]Item, 20)
	for i := range items {
		items[i] = Item{Label: fmt.Sprintf("item%d", i)}
	}
	m := newModel(items, nil)
	m.height = viewOverhead + 5 // 5 visible rows

	// Cursor within first page — viewport starts at 0.
	m.cursor = 3
	if got := m.viewportStart(); got != 0 {
		t.Errorf("cursor=3, vis=5: viewportStart=%d, want 0", got)
	}

	// Cursor at exactly vis-1 — still starts at 0.
	m.cursor = 4
	if got := m.viewportStart(); got != 0 {
		t.Errorf("cursor=4, vis=5: viewportStart=%d, want 0", got)
	}

	// Cursor one past the first visible page — viewport must scroll.
	m.cursor = 5
	if got := m.viewportStart(); got != 1 {
		t.Errorf("cursor=5, vis=5: viewportStart=%d, want 1", got)
	}

	// Cursor at the last item — viewport shows last 5 items.
	m.cursor = 19
	if got := m.viewportStart(); got != 15 {
		t.Errorf("cursor=19, vis=5: viewportStart=%d, want 15", got)
	}
}
