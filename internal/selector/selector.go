// Package selector provides an interactive fuzzy multi-select TUI built on
// bubbletea.  The user types to filter the list, navigates with ↑/↓, toggles
// individual items with SPACE, selects/deselects all visible items with A, and
// confirms with ENTER.  Ctrl+R refreshes the list from the source.
package selector

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Item is a single selectable row. Label is the string shown and matched
// against. Key is a stable identifier used to re-apply selections after a
// Ctrl+R refresh; if empty, the selection for that item is not preserved.
type Item struct {
	Label string
	Key   string
}

// refreshMsg is sent back to the model when a background refresh completes.
type refreshMsg struct {
	items []Item
	err   error
}

type model struct {
	items       []Item
	filtered    []int        // positions in items[] that pass the current query
	selected    map[int]bool // keyed by position in items[]
	cursor      int          // position in filtered[]
	query       string
	done        bool
	quit        bool
	refreshFn   func() ([]Item, error)
	refreshing  bool
	refreshErr  error
	lastRefresh time.Time
	height      int // terminal height (0 = unknown)
	width       int // terminal width (0 = unknown)
}

func newModel(items []Item, refreshFn func() ([]Item, error)) model {
	m := model{
		items:       items,
		selected:    make(map[int]bool),
		refreshFn:   refreshFn,
		lastRefresh: time.Now(),
	}
	m.refilter()
	return m
}

// refilter rebuilds m.filtered so it holds the item indices that match m.query.
func (m *model) refilter() {
	filtered := make([]int, 0, len(m.items))
	for i, item := range m.items {
		if fuzzyMatch(m.query, item.Label) {
			filtered = append(filtered, i)
		}
	}
	m.filtered = filtered
	// Keep cursor in bounds.
	switch {
	case len(m.filtered) == 0:
		m.cursor = 0
	case m.cursor >= len(m.filtered):
		m.cursor = len(m.filtered) - 1
	}
}

// fuzzyMatch returns true if query is a case-insensitive substring of target.
func fuzzyMatch(query, target string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(target), strings.ToLower(query))
}

func (m model) Init() tea.Cmd { return nil }

// viewOverhead is the number of terminal rows consumed by the header and
// footer, i.e. lines that are not list items:
//
//	blank line + "Search:" line + blank line  (header, 3 rows)
//	blank line + footer line                  (footer, 2 rows)
const viewOverhead = 5

// visibleLines returns how many list-item rows can be rendered within the
// current terminal height.  Falls back to showing all items when the terminal
// height is not yet known (zero).
func (m model) visibleLines() int {
	if m.height == 0 {
		return len(m.filtered)
	}
	vis := m.height - viewOverhead
	if vis < 1 {
		vis = 1
	}
	return vis
}

// viewportStart returns the index (into m.filtered) of the first item that
// should be rendered, keeping m.cursor always within the visible window.
func (m model) viewportStart() int {
	vis := m.visibleLines()
	if m.cursor < vis {
		return 0
	}
	start := m.cursor - vis + 1
	maxStart := len(m.filtered) - vis
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	return start
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		return m, nil

	case refreshMsg:
		m.refreshing = false
		if msg.err != nil {
			m.refreshErr = msg.err
			return m, nil
		}
		// Restore selections for items whose Key is still present.
		oldSelected := make(map[string]bool)
		for idx, sel := range m.selected {
			if sel && idx < len(m.items) && m.items[idx].Key != "" {
				oldSelected[m.items[idx].Key] = true
			}
		}
		m.items = msg.items
		m.selected = make(map[int]bool)
		for i, item := range m.items {
			if item.Key != "" && oldSelected[item.Key] {
				m.selected[i] = true
			}
		}
		m.refreshErr = nil
		m.lastRefresh = time.Now()
		m.refilter()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {

		case tea.KeyCtrlC:
			m.quit = true
			return m, tea.Quit

		case tea.KeyEsc:
			if m.query != "" {
				// First Escape clears the search without exiting.
				m.query = ""
				m.refilter()
			} else {
				m.quit = true
				return m, tea.Quit
			}

		case tea.KeyEnter:
			m.done = true
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}

		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case tea.KeySpace:
			if len(m.filtered) > 0 {
				idx := m.filtered[m.cursor]
				m.selected[idx] = !m.selected[idx]
			}

		case tea.KeyBackspace:
			if len(m.query) > 0 {
				runes := []rune(m.query)
				m.query = string(runes[:len(runes)-1])
				m.refilter()
			}

		case tea.KeyCtrlA:
			// Toggle all visible items.  If any are unselected, select all;
			// if all are already selected, deselect all.
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

		case tea.KeyCtrlR:
			if m.refreshFn != nil && !m.refreshing {
				m.refreshing = true
				m.refreshErr = nil
				fn := m.refreshFn
				return m, func() tea.Msg {
					items, err := fn()
					return refreshMsg{items: items, err: err}
				}
			}

		case tea.KeyRunes:
			m.query += string(msg.Runes)
			m.refilter()
		}
	}
	return m, nil
}

func (m model) View() string {
	var sb strings.Builder

	// Search bar.
	fmt.Fprintf(&sb, "\n  Search: %s▌\n\n", m.query)

	// Job rows — only render the viewport slice so the line count returned by
	// View() stays within the terminal height and bubbletea can redraw without
	// ghosting old content.
	if len(m.filtered) == 0 {
		sb.WriteString("  (no matches)\n")
	} else {
		start := m.viewportStart()
		end := start + m.visibleLines()
		if end > len(m.filtered) {
			end = len(m.filtered)
		}
		for i := start; i < end; i++ {
			fi := m.filtered[i]
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			check := "[ ]"
			if m.selected[fi] {
				check = "[x]"
			}
			fmt.Fprintf(&sb, "  %s%s  %s\n", cursor, check, m.items[fi].Label)
		}
	}

	// Footer.
	nSel := 0
	for _, v := range m.selected {
		if v {
			nSel++
		}
	}

	var refreshStatus string
	switch {
	case m.refreshing:
		refreshStatus = "  [refreshing...]"
	case m.refreshErr != nil:
		refreshStatus = fmt.Sprintf("  [refresh error: %v]", m.refreshErr)
	case !m.lastRefresh.IsZero():
		refreshStatus = fmt.Sprintf("  [last refresh: %s]", m.lastRefresh.Local().Format("15:04:05"))
	}

	fmt.Fprintf(&sb, "\n  %d/%d shown  %d selected  |  ↑↓ navigate  SPACE toggle  Ctrl+A all  Ctrl+R refresh  ENTER confirm  ESC cancel%s\n",
		len(m.filtered), len(m.items), nSel, refreshStatus)

	return sb.String()
}

// Run presents the interactive fuzzy multi-select UI and returns the indices
// (into the original items slice) that the user selected.
// Returns nil without an error if the user cancels (ESC or Ctrl+C).
// refreshFn, if non-nil, is called when the user presses Ctrl+R to reload
// the item list; previously-selected items are re-selected by Key.
func Run(items []Item, refreshFn func() ([]Item, error)) ([]int, error) {
	if len(items) == 0 {
		return nil, nil
	}
	p := tea.NewProgram(newModel(items, refreshFn), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("selector: %w", err)
	}

	fm := final.(model)
	if fm.quit || !fm.done {
		return nil, nil
	}

	result := make([]int, 0, len(fm.selected))
	for idx, sel := range fm.selected {
		if sel {
			result = append(result, idx)
		}
	}
	return result, nil
}
