package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

// Screen represents the current view in the TUI
type Screen int

const (
	QueryBuilderScreen Screen = iota
	ResultsScreen
	HelpScreen
)

// QueryField represents a configurable query parameter
type QueryField int

const (
	CmdField QueryField = iota
	MapField
	TypeField
	PatternField
	FormatField
	OutputField
)

// Model represents the TUI state
type Model struct {
	// Core state
	Screen   Screen
	Registry *t6assets.Registry
	ZonePath string
	UseCache bool

	// Query configuration
	Query       QueryConfig
	ActiveField QueryField
	Fields      []QueryField
	FieldInputs map[QueryField]*textinput.Model

	// Results
	Results         []*t6assets.Asset
	FilteredResults []*t6assets.Asset
	Viewport        viewport.Model
	Cursor          int
	SearchInput     *textinput.Model
	IsSearching     bool

	// Status
	StatusMessage string
	IsLoading     bool
	Error         error

	// Styles
	Styles Styles
}

// QueryConfig holds the query parameters
type QueryConfig struct {
	Cmd         string
	Map         string
	Type        string
	Pattern     string
	Format      string
	Output      string
	SortBy      string
	IgnoreCase  bool
	UseWildcard bool
}

// Styles holds lipgloss styles
type Styles struct {
	Title          lipgloss.Style
	Subtitle       lipgloss.Style
	FieldLabel     lipgloss.Style
	FieldValue     lipgloss.Style
	FieldActive    lipgloss.Style
	FieldInactive  lipgloss.Style
	HelpText       lipgloss.Style
	StatusBar      lipgloss.Style
	ErrorText      lipgloss.Style
	ResultItem     lipgloss.Style
	ResultSelected lipgloss.Style
	SearchBox      lipgloss.Style
}

// DefaultStyles creates default lipgloss styles
func DefaultStyles() Styles {
	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginLeft(2).
			MarginBottom(1),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginLeft(2).
			MarginBottom(1),
		FieldLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ADD8")).
			Bold(true).
			Width(12),
		FieldValue: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),
		FieldActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#7D56F4")).
			Bold(true),
		FieldInactive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")),
		HelpText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")),
		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#333333")).
			PaddingLeft(1).
			PaddingRight(1),
		ErrorText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true),
		ResultItem: lipgloss.NewStyle().
			PaddingLeft(2),
		ResultSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2),
		SearchBox: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#444444")).
			PaddingLeft(1).
			PaddingRight(1),
	}
}

// NewModel creates a new TUI model
func NewModel(zonePath string, useCache bool) Model {
	// Initialize query fields
	fields := []QueryField{CmdField, MapField, TypeField, PatternField, FormatField, OutputField}

	// Initialize text inputs for each field
	fieldInputs := make(map[QueryField]*textinput.Model)

	for _, field := range fields {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Width = 40

		switch field {
		case CmdField:
			ti.Placeholder = "index, list, search, export"
			ti.SetValue("list")
		case MapField:
			ti.Placeholder = "zm_tomb, zm_prison (comma-separated)"
		case TypeField:
			ti.Placeholder = "weapon, perk, xmodel, material, image"
		case PatternField:
			ti.Placeholder = "raygun, upgraded (comma-separated, ! to exclude)"
		case FormatField:
			ti.Placeholder = "plain, json, csv, gsc"
			ti.SetValue("plain")
		case OutputField:
			ti.Placeholder = "stdout or path/to/file"
		}

		fieldInputs[field] = &ti
	}

	// Initialize search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search in results..."
	searchInput.Width = 40

	// Initialize viewport for results
	vp := viewport.New(80, 20)

	return Model{
		Screen:          QueryBuilderScreen,
		Registry:        t6assets.NewRegistry(),
		ZonePath:        zonePath,
		UseCache:        useCache,
		Query:           QueryConfig{},
		ActiveField:     CmdField,
		Fields:          fields,
		FieldInputs:     fieldInputs,
		Results:         []*t6assets.Asset{},
		FilteredResults: []*t6assets.Asset{},
		Viewport:        vp,
		Cursor:          0,
		SearchInput:     &searchInput,
		IsSearching:     false,
		StatusMessage:   "Configure your query and press Enter to execute",
		IsLoading:       false,
		Styles:          DefaultStyles(),
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	m.FieldInputs[CmdField].Focus()
	return nil
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// First, update the active text input to capture typed characters
		if m.Screen == QueryBuilderScreen {
			activeInput := m.FieldInputs[m.ActiveField]
			newInput, cmd := activeInput.Update(msg)
			*m.FieldInputs[m.ActiveField] = newInput
			cmds = append(cmds, cmd)
		} else if m.Screen == ResultsScreen && m.IsSearching {
			newInput, cmd := m.SearchInput.Update(msg)
			*m.SearchInput = newInput
			cmds = append(cmds, cmd)
			m.filterResults()
		}

		// Then handle navigation keys
		switch m.Screen {
		case QueryBuilderScreen:
			return m.updateQueryBuilder(msg)
		case ResultsScreen:
			return m.updateResultsScreen(msg)
		case HelpScreen:
			return m.updateHelpScreen(msg)
		}

	case tea.WindowSizeMsg:
		m.Viewport.Width = msg.Width - 4
		m.Viewport.Height = msg.Height - 8

	case LoadCompleteMsg:
		m.IsLoading = false
		if msg.Error != nil {
			m.Error = msg.Error
			m.StatusMessage = fmt.Sprintf("Error: %v", msg.Error)
		} else {
			m.Results = msg.Assets
			m.FilteredResults = msg.Assets
			m.Cursor = 0
			m.StatusMessage = fmt.Sprintf("Found %d results", len(m.Results))
			m.Screen = ResultsScreen
		}
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m Model) View() string {
	switch m.Screen {
	case QueryBuilderScreen:
		return m.viewQueryBuilder()
	case ResultsScreen:
		return m.viewResultsScreen()
	case HelpScreen:
		return m.viewHelpScreen()
	default:
		return "Unknown screen"
	}
}

// updateQueryBuilder handles key events in query builder screen
func (m Model) updateQueryBuilder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab", "down":
		// Move to next field (only use Tab/Down arrows, not j/k)
		m.ActiveField = m.Fields[(int(m.ActiveField)+1)%len(m.Fields)]
		m.FieldInputs[m.ActiveField].Focus()
		return m, nil

	case "shift+tab", "up":
		// Move to previous field (only use Shift+Tab/Up arrows, not k)
		idx := int(m.ActiveField) - 1
		if idx < 0 {
			idx = len(m.Fields) - 1
		}
		m.ActiveField = m.Fields[idx]
		m.FieldInputs[m.ActiveField].Focus()
		return m, nil

	case "enter":
		// Execute query
		return m, m.executeQuery()

	case "?", "h":
		// Show help
		m.Screen = HelpScreen
		return m, nil

	case "ctrl+l":
		// Clear all fields
		for _, field := range m.Fields {
			m.FieldInputs[field].SetValue("")
		}
		m.FieldInputs[CmdField].SetValue("list")
		m.FieldInputs[FormatField].SetValue("plain")
		return m, nil
	}

	return m, nil
}

// updateResultsScreen handles key events in results screen
func (m Model) updateResultsScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.IsSearching {
		switch msg.String() {
		case "esc", "ctrl+c":
			m.IsSearching = false
			m.SearchInput.SetValue("")
			m.filterResults()
			return m, nil
		case "enter":
			m.IsSearching = false
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "b":
		// Back to query builder
		m.Screen = QueryBuilderScreen
		m.Error = nil
		return m, nil

	case "j", "down":
		// Move down
		if m.Cursor < len(m.FilteredResults)-1 {
			m.Cursor++
			m.updateViewport()
		}

	case "k", "up":
		// Move up
		if m.Cursor > 0 {
			m.Cursor--
			m.updateViewport()
		}

	case "g":
		// Go to top
		m.Cursor = 0
		m.updateViewport()

	case "G":
		// Go to bottom
		m.Cursor = len(m.FilteredResults) - 1
		if m.Cursor < 0 {
			m.Cursor = 0
		}
		m.updateViewport()

	case "ctrl+d":
		// Half page down
		m.Cursor += m.Viewport.Height / 2
		if m.Cursor >= len(m.FilteredResults) {
			m.Cursor = len(m.FilteredResults) - 1
			if m.Cursor < 0 {
				m.Cursor = 0
			}
		}
		m.updateViewport()

	case "ctrl+u":
		// Half page up
		m.Cursor -= m.Viewport.Height / 2
		if m.Cursor < 0 {
			m.Cursor = 0
		}
		m.updateViewport()

	case "/":
		// Start searching
		m.IsSearching = true
		return m, m.SearchInput.Focus()

	case "n":
		// Next search result
		m.findNext()

	case "N":
		// Previous search result
		m.findPrevious()

	case "y":
		// Copy current result to clipboard (would need implementation)
		if m.Cursor < len(m.FilteredResults) {
			asset := m.FilteredResults[m.Cursor]
			m.StatusMessage = fmt.Sprintf("Copied: %s", asset.Name)
		}

	case "?", "h":
		// Show help
		m.Screen = HelpScreen
		return m, nil
	}

	return m, nil
}

// updateHelpScreen handles key events in help screen
func (m Model) updateHelpScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "?", "h":
		// Return to previous screen
		if len(m.Results) > 0 {
			m.Screen = ResultsScreen
		} else {
			m.Screen = QueryBuilderScreen
		}
		return m, nil
	}
	return m, nil
}

// viewQueryBuilder renders the query builder screen
func (m Model) viewQueryBuilder() string {
	var b strings.Builder

	b.WriteString(m.Styles.Title.Render("T6 Asset Browser"))
	b.WriteString("\n")
	b.WriteString(m.Styles.Subtitle.Render("Configure your query below"))
	b.WriteString("\n\n")

	// Field labels and values
	fieldLabels := map[QueryField]string{
		CmdField:     "Command",
		MapField:     "Map",
		TypeField:    "Type",
		PatternField: "Pattern",
		FormatField:  "Format",
		OutputField:  "Output",
	}

	for _, field := range m.Fields {
		label := fieldLabels[field]
		input := m.FieldInputs[field]

		var fieldStyle lipgloss.Style
		if field == m.ActiveField {
			fieldStyle = m.Styles.FieldActive
			b.WriteString("► ")
		} else {
			fieldStyle = m.Styles.FieldInactive
			b.WriteString("  ")
		}

		b.WriteString(m.Styles.FieldLabel.Render(label + ":"))
		b.WriteString(" ")

		if field == m.ActiveField {
			b.WriteString(fieldStyle.Render(input.View()))
		} else {
			displayValue := input.Value()
			if displayValue == "" {
				displayValue = input.Placeholder
				b.WriteString(m.Styles.HelpText.Render(displayValue))
			} else {
				b.WriteString(fieldStyle.Render(displayValue))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Status/error messages
	if m.Error != nil {
		b.WriteString(m.Styles.ErrorText.Render(fmt.Sprintf("Error: %v", m.Error)))
		b.WriteString("\n")
	} else {
		b.WriteString(m.Styles.StatusBar.Render(m.StatusMessage))
	}

	b.WriteString("\n\n")
	b.WriteString(m.Styles.HelpText.Render("Press Enter to execute • Tab/↓/↑ to navigate • ? for help • q to quit"))

	return b.String()
}

// viewResultsScreen renders the results screen
func (m Model) viewResultsScreen() string {
	var b strings.Builder

	b.WriteString(m.Styles.Title.Render("Results"))
	b.WriteString("\n")

	if m.IsSearching {
		b.WriteString(m.Styles.SearchBox.Render("/" + m.SearchInput.View()))
		b.WriteString("\n")
	}

	// Build results list
	var content strings.Builder
	start := m.Viewport.YOffset
	end := start + m.Viewport.Height
	if end > len(m.FilteredResults) {
		end = len(m.FilteredResults)
	}
	if start < 0 {
		start = 0
	}

	for i := start; i < end; i++ {
		if i >= len(m.FilteredResults) {
			break
		}

		asset := m.FilteredResults[i]
		line := fmt.Sprintf("[%s] %s (from %s)\n", asset.Type, asset.Name, asset.Source)

		if i == m.Cursor {
			content.WriteString(m.Styles.ResultSelected.Render("> " + line))
		} else {
			content.WriteString(m.Styles.ResultItem.Render("  " + line))
		}
	}

	// Update viewport content
	m.Viewport.SetContent(content.String())
	b.WriteString(m.Viewport.View())

	// Status bar
	b.WriteString("\n")
	status := fmt.Sprintf("%d/%d results • Cursor: %d", len(m.FilteredResults), len(m.Results), m.Cursor+1)
	if m.StatusMessage != "" && !m.IsSearching {
		status = m.StatusMessage
	}
	b.WriteString(m.Styles.StatusBar.Render(status))

	b.WriteString("\n")
	b.WriteString(m.Styles.HelpText.Render("j/↓/k/↑ to navigate • g/G for top/bottom • / to search • y to copy • b/esc back • ? help • q quit"))

	return b.String()
}

// viewHelpScreen renders the help screen
func (m Model) viewHelpScreen() string {
	var b strings.Builder

	b.WriteString(m.Styles.Title.Render("Help"))
	b.WriteString("\n\n")

	helpText := `
Query Builder Keys:
  Tab/↓ or Shift+Tab/↑     Navigate between fields
  Enter                    Execute the query
  Ctrl+L                   Clear all fields
  ? or h                   Show this help
  q or Ctrl+C              Quit

Results Screen Keys:
  j/↓ or k/↑               Move cursor down/up
  g                        Go to first result
  G                        Go to last result
  Ctrl+D                   Half page down
  Ctrl+U                   Half page up
  /                        Search in results
  n/N                      Next/previous search result
  y                        Copy current item name
  b or Esc                 Back to query builder
  ? or h                   Show this help
  q or Ctrl+C              Quit

Query Syntax:
  -cmd:    index, list, search, export
  -map:    Comma-separated map names (zm_tomb, zm_prison)
  -type:   Comma-separated types (weapon, perk, xmodel)
  -pattern: Comma-separated patterns with AND logic
           Use ! prefix to exclude (e.g., upgraded,!staff)
  -format: plain, json, csv, gsc
  -output: File path or leave empty for stdout
`

	b.WriteString(m.Styles.HelpText.Render(helpText))
	b.WriteString("\n")
	b.WriteString(m.Styles.StatusBar.Render("Press q, esc, or ?/h to return"))

	return b.String()
}

// updateViewport updates the viewport position based on cursor
func (m *Model) updateViewport() {
	if m.Cursor < m.Viewport.YOffset {
		m.Viewport.YOffset = m.Cursor
	} else if m.Cursor >= m.Viewport.YOffset+m.Viewport.Height {
		m.Viewport.YOffset = m.Cursor - m.Viewport.Height + 1
	}
}

// filterResults filters results based on search input
func (m *Model) filterResults() {
	searchTerm := strings.ToLower(m.SearchInput.Value())
	if searchTerm == "" {
		m.FilteredResults = m.Results
		return
	}

	var filtered []*t6assets.Asset
	for _, asset := range m.Results {
		if strings.Contains(strings.ToLower(asset.Name), searchTerm) ||
			strings.Contains(strings.ToLower(asset.Type.String()), searchTerm) ||
			strings.Contains(strings.ToLower(asset.Source), searchTerm) {
			filtered = append(filtered, asset)
		}
	}
	m.FilteredResults = filtered
	m.Cursor = 0
	m.Viewport.YOffset = 0
}

// findNext finds the next occurrence of search term
func (m *Model) findNext() {
	searchTerm := strings.ToLower(m.SearchInput.Value())
	if searchTerm == "" {
		return
	}

	for i := m.Cursor + 1; i < len(m.FilteredResults); i++ {
		asset := m.FilteredResults[i]
		if strings.Contains(strings.ToLower(asset.Name), searchTerm) {
			m.Cursor = i
			m.updateViewport()
			return
		}
	}
}

// findPrevious finds the previous occurrence of search term
func (m *Model) findPrevious() {
	searchTerm := strings.ToLower(m.SearchInput.Value())
	if searchTerm == "" {
		return
	}

	for i := m.Cursor - 1; i >= 0; i-- {
		asset := m.FilteredResults[i]
		if strings.Contains(strings.ToLower(asset.Name), searchTerm) {
			m.Cursor = i
			m.updateViewport()
			return
		}
	}
}

// LoadCompleteMsg is sent when query execution completes
type LoadCompleteMsg struct {
	Assets []*t6assets.Asset
	Error  error
}

// executeQuery executes the current query and returns a command
func (m Model) executeQuery() tea.Cmd {
	return func() tea.Msg {
		// Parse query configuration
		m.Query.Cmd = m.FieldInputs[CmdField].Value()
		m.Query.Map = m.FieldInputs[MapField].Value()
		m.Query.Type = m.FieldInputs[TypeField].Value()
		m.Query.Pattern = m.FieldInputs[PatternField].Value()
		m.Query.Format = m.FieldInputs[FormatField].Value()
		m.Query.Output = m.FieldInputs[OutputField].Value()

		// For simplicity, we only support list and search commands in TUI
		// Export would require file handling which is better suited for CLI
		switch m.Query.Cmd {
		case "list", "search":
			// Execute the query
			return m.runQuery()
		default:
			return LoadCompleteMsg{
				Error: fmt.Errorf("command '%s' not supported in TUI (use list or search)", m.Query.Cmd),
			}
		}
	}
}

// runQuery runs the actual query and returns results
func (m Model) runQuery() tea.Msg {
	results, err := ExecuteQuery(m.ZonePath, m.Query, m.UseCache)
	return LoadCompleteMsg{
		Assets: results,
		Error:  err,
	}
}
