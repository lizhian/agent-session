package picker

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lizhian/agent-session/internal/render"
)

type listViewportMode int

const (
	listViewportFollowSelection listViewportMode = iota
	listViewportCenterSelection
)

type listRow struct {
	Text     string
	Selected bool
	Focused  bool
	Color    string
}

type listView struct {
	Width                int
	Height               int
	Header               []string
	Rows                 []listRow
	Footer               []string
	UseColor             bool
	DefaultSelectedColor string
	ViewportMode         listViewportMode
	EmptyMessage         string
}

func (v listView) Render() string {
	if len(v.Rows) == 0 {
		lines := append([]string(nil), v.Header...)
		if v.EmptyMessage != "" {
			lines = append(lines, render.FitLine(v.EmptyMessage, v.Width))
		}
		lines = append(lines, v.Footer...)
		lines = append(lines, renderHelp(v.Width, helpKeyMapForView(helpKindTable)))
		return renderViewport(v.Width, v.Height, strings.Join(lines, "\n"), 0)
	}

	selectedIndex := v.selectedIndex()
	bodyHeight := max(1, v.Height-len(v.Header)-len(v.Footer)-1)
	body := renderTable(v.Width, bodyHeight, v.Rows, selectedIndex, v.ViewportMode, v.UseColor)
	lines := append([]string(nil), v.Header...)
	lines = append(lines, body)
	lines = append(lines, v.Footer...)
	lines = append(lines, renderHelp(v.Width, helpKeyMapForView(helpKindTable)))
	return strings.Join(lines, "\n")
}

func (v listView) selectedIndex() int {
	for i, row := range v.Rows {
		if row.Focused {
			return i
		}
	}
	for i, row := range v.Rows {
		if row.Selected {
			return i
		}
	}
	return 0
}

func listWindowStart(itemCount int, selectedIndex int, maxRows int, mode listViewportMode) int {
	if itemCount <= 0 || maxRows <= 0 || itemCount <= maxRows {
		return 0
	}
	idx := render.ClampSelectedIndex(selectedIndex, itemCount)
	var start int
	if mode == listViewportCenterSelection {
		start = idx - maxRows/2
	} else {
		start = idx - maxRows + 1
	}
	if start < 0 {
		return 0
	}
	maxStart := itemCount - maxRows
	if start > maxStart {
		return maxStart
	}
	return start
}

type menuListView struct {
	Width        int
	Height       int
	Header       []string
	Rows         []listRow
	Selected     int
	Footer       []string
	EmptyMessage string
	UseColor     bool
}

func (v menuListView) Render() string {
	if len(v.Rows) == 0 {
		lines := append([]string(nil), v.Header...)
		if v.EmptyMessage != "" {
			lines = append(lines, render.FitLine(v.EmptyMessage, v.Width))
		}
		lines = append(lines, v.Footer...)
		lines = append(lines, renderHelp(v.Width, helpKeyMapForView(helpKindMenu)))
		return renderViewport(v.Width, v.Height, strings.Join(lines, "\n"), 0)
	}
	bodyHeight := max(1, v.Height-len(v.Header)-len(v.Footer)-1)
	body := renderMenuList(v.Width, bodyHeight, v.Rows, v.Selected, v.UseColor)
	lines := append([]string(nil), v.Header...)
	lines = append(lines, body)
	lines = append(lines, v.Footer...)
	lines = append(lines, renderHelp(v.Width, helpKeyMapForView(helpKindMenu)))
	return strings.Join(lines, "\n")
}

func renderTable(width int, height int, rows []listRow, selectedIndex int, viewportMode listViewportMode, useColor bool) string {
	colWidth := max(1, width-2)
	maxRows := max(1, height)
	start := listWindowStart(len(rows), selectedIndex, maxRows, viewportMode)
	visibleRows := rows[start:min(start+maxRows, len(rows))]
	tableRows := make([]table.Row, 0, len(visibleRows))
	for _, row := range visibleRows {
		tableRows = append(tableRows, table.Row{row.Text})
	}
	styles := table.DefaultStyles()
	styles.Header = lipgloss.NewStyle()
	styles.Cell = lipgloss.NewStyle()
	if useColor {
		styles.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	} else {
		styles.Selected = lipgloss.NewStyle()
	}
	t := table.New(
		table.WithColumns([]table.Column{{Title: "", Width: colWidth}}),
		table.WithRows(tableRows),
		table.WithWidth(width),
		table.WithHeight(height+1),
		table.WithFocused(true),
		table.WithStyles(styles),
	)
	t.SetCursor(max(0, selectedIndex-start))
	lines := strings.Split(t.View(), "\n")
	if len(lines) > 1 {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

type menuItem struct {
	text string
}

func (i menuItem) FilterValue() string {
	return i.text
}

type menuDelegate struct {
	useColor bool
}

func (d menuDelegate) Height() int {
	return 1
}

func (d menuDelegate) Spacing() int {
	return 0
}

func (d menuDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d menuDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(menuItem)
	if !ok {
		return
	}
	text := i.text
	if index == m.Index() {
		text = render.Colorize(text, render.ANSISelected, d.useColor)
	}
	_, _ = fmt.Fprint(w, text)
}

func renderMenuList(width int, height int, rows []listRow, selectedIndex int, useColor bool) string {
	items := make([]list.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, menuItem{text: row.Text})
	}
	m := list.New(items, menuDelegate{useColor: useColor}, width, height)
	m.SetShowTitle(false)
	m.SetShowFilter(false)
	m.SetFilteringEnabled(false)
	m.SetShowStatusBar(false)
	m.SetShowPagination(false)
	m.SetShowHelp(false)
	m.DisableQuitKeybindings()
	m.Select(selectedIndex)
	return m.View()
}

type helpKind int

const (
	helpKindTable helpKind = iota
	helpKindMenu
	helpKindPreview
	helpKindInput
)

type pickerHelpKeyMap struct {
	bindings []key.Binding
}

func (m pickerHelpKeyMap) ShortHelp() []key.Binding {
	return m.bindings
}

func (m pickerHelpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.bindings}
}

func helpKeyMapForView(kind helpKind) pickerHelpKeyMap {
	switch kind {
	case helpKindPreview:
		return pickerHelpKeyMap{bindings: []key.Binding{
			key.NewBinding(key.WithKeys("esc", "space"), key.WithHelp("esc/space", "back")),
			key.NewBinding(key.WithKeys("↑/↓", "k/j"), key.WithHelp("↑/↓", "scroll")),
		}}
	case helpKindInput:
		return pickerHelpKeyMap{bindings: []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "install")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		}}
	case helpKindMenu:
		return pickerHelpKeyMap{bindings: []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
			key.NewBinding(key.WithKeys("↑/↓", "k/j"), key.WithHelp("↑/↓", "move")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		}}
	default:
		return pickerHelpKeyMap{bindings: []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
			key.NewBinding(key.WithKeys("↑/↓", "k/j"), key.WithHelp("↑/↓", "move")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "permission")),
		}}
	}
}

func renderHelp(width int, keyMap pickerHelpKeyMap) string {
	h := help.New()
	h.Width = width
	return h.View(keyMap)
}

func renderViewport(width int, height int, content string, yOffset int) string {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	vp := viewport.New(width, height)
	vp.SetContent(content)
	vp.SetYOffset(yOffset)
	return vp.View()
}
