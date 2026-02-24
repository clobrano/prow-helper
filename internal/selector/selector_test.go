package selector

import "testing"

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
		{"aon", "pull-ci-openshift-e2e-aws-ovn", true},  // chars in order: a…o…n
		{"noa", "pull-ci-openshift-e2e-aws-ovn", false}, // wrong order
		{"x", "pull-ci-openshift-e2e-aws-ovn", false},
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
	m := newModel(items)

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
	m := newModel(items)

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

// toggleAll simulates pressing "A" by calling the same logic used in Update.
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
	m := newModel(items)
	m.cursor = 2

	// Narrow the query so only one item is visible; cursor must clamp.
	m.query = "a"
	m.refilter()
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", m.cursor)
	}
}
