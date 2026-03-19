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

// normalizeString removes leading/trailing whitespace and normalizes indentation
func normalizeString(s string) string {
	return strings.TrimSpace(s)
}

// normalizeMultiline normalizes a multi-line string by trimming each line
func normalizeMultiline(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

// Screen represents the current view in the TUI
type Screen int

const (
	QueryBuilderScreen Screen = iota
	ResultsScreen
	HelpScreen
)

// Mode represents vim-like modes
type Mode int

const (
	NormalMode Mode = iota
	InsertMode
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
	Mode     Mode
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

	// Terminal dimensions
	Width  int
	Height int

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
	LoadingText    lipgloss.Style
	ResultItem     lipgloss.Style
	ResultSelected lipgloss.Style
	SearchBox      lipgloss.Style
	Container      lipgloss.Style
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
		LoadingText: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Background(lipgloss.Color("#333333")).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1),
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
		Container: lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingRight(2),
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

	// Initialize viewport for results - will be resized on first WindowSizeMsg
	vp := viewport.New(80, 20)

	return Model{
		Screen:          QueryBuilderScreen,
		Mode:            NormalMode,
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
		StatusMessage:   "Press 'i' to edit field, '?' for help",
		IsLoading:       false,
		Styles:          DefaultStyles(),
		Width:           80,
		Height:          24,
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
		// Handle screen-specific updates
		switch m.Screen {
		case QueryBuilderScreen:
			return m.updateQueryBuilder(msg)
		case ResultsScreen:
			return m.updateResultsScreen(msg)
		case HelpScreen:
			return m.updateHelpScreen(msg)
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Viewport.Width = msg.Width - 4
		// Reserve space for header, status bar, and help text
		m.Viewport.Height = msg.Height - 10
		if m.Viewport.Height < 5 {
			m.Viewport.Height = 5
		}

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
			m.Mode = NormalMode
		}

	case ExportCompleteMsg:
		m.IsLoading = false
		if msg.Error != nil {
			m.Error = msg.Error
			m.StatusMessage = fmt.Sprintf("Export error: %v", msg.Error)
		} else {
			m.StatusMessage = fmt.Sprintf("Exported %d assets to %s", msg.Count, msg.Filename)
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
	// In INSERT mode, pass keys to text input (except Esc to exit)
	if m.Mode == InsertMode {
		switch msg.String() {
		case "esc":
			m.Mode = NormalMode
			m.StatusMessage = "NORMAL mode - press 'i' to edit, '?' for help"
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		default:
			// Pass key to text input
			activeInput := m.FieldInputs[m.ActiveField]
			newInput, cmd := activeInput.Update(msg)
			*m.FieldInputs[m.ActiveField] = newInput
			return m, cmd
		}
	}

	// In NORMAL mode, handle vim navigation
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "i", "I":
		// Enter INSERT mode
		m.Mode = InsertMode
		m.StatusMessage = "INSERT mode - type to edit, Esc for Normal mode"
		m.FieldInputs[m.ActiveField].Focus()
		return m, nil

	case "j", "down":
		// Move to next field
		m.ActiveField = m.Fields[(int(m.ActiveField)+1)%len(m.Fields)]
		return m, nil

	case "k", "up":
		// Move to previous field
		idx := int(m.ActiveField) - 1
		if idx < 0 {
			idx = len(m.Fields) - 1
		}
		m.ActiveField = m.Fields[idx]
		return m, nil

	case "tab":
		// Alternative navigation: next field
		m.ActiveField = m.Fields[(int(m.ActiveField)+1)%len(m.Fields)]
		return m, nil

	case "shift+tab":
		// Alternative navigation: previous field
		idx := int(m.ActiveField) - 1
		if idx < 0 {
			idx = len(m.Fields) - 1
		}
		m.ActiveField = m.Fields[idx]
		return m, nil

	case "enter":
		// Execute query
		m.IsLoading = true
		m.StatusMessage = "Loading..."
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
	// Calculate available width for input fields
	availableWidth := m.Width - 20 // Account for margins, label (12), padding, etc.
	if availableWidth < 20 {
		availableWidth = 20
	}
	if availableWidth > 60 {
		availableWidth = 60
	}

	// Update input widths
	for _, field := range m.Fields {
		m.FieldInputs[field].Width = availableWidth
	}

	var b strings.Builder

	// Show mode indicator in title
	modeStr := ""
	if m.Mode == NormalMode {
		modeStr = " [NORMAL]"
	} else {
		modeStr = " [INSERT]"
	}
	b.WriteString(m.Styles.Title.Render("T6 Asset Browser" + modeStr))
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
			// Truncate input view if too long
			inputView := normalizeString(input.View())
			if len(inputView) > availableWidth {
				inputView = inputView[:availableWidth-3] + "..."
			}
			b.WriteString(fieldStyle.Render(inputView))
		} else {
			displayValue := normalizeString(input.Value())
			if displayValue == "" {
				displayValue = normalizeString(input.Placeholder)
				b.WriteString(m.Styles.HelpText.Render(displayValue))
			} else {
				// Truncate display value if too long
				if len(displayValue) > availableWidth {
					displayValue = displayValue[:availableWidth-3] + "..."
				}
				b.WriteString(fieldStyle.Render(displayValue))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Status/error/loading messages
	if m.IsLoading {
		// Show prominent loading indicator
		loadingMsg := "⏳ Loading..."
		if m.Query.Cmd == "export" {
			loadingMsg = "⏳ Exporting assets..."
		}
		b.WriteString(m.Styles.LoadingText.Render(loadingMsg))
		b.WriteString("\n")
	} else if m.Error != nil {
		errorMsg := normalizeString(fmt.Sprintf("❌ Error: %v", m.Error))
		if len(errorMsg) > m.Width-4 {
			errorMsg = errorMsg[:m.Width-7] + "..."
		}
		b.WriteString(m.Styles.ErrorText.Render(errorMsg))
		b.WriteString("\n")
	} else {
		statusMsg := normalizeString(m.StatusMessage)
		if len(statusMsg) > m.Width-4 {
			statusMsg = statusMsg[:m.Width-7] + "..."
		}
		b.WriteString(m.Styles.StatusBar.Render(statusMsg))
	}

	b.WriteString("\n")
	if m.Mode == NormalMode && !m.IsLoading {
		helpText := "i=insert • j/k=navigate • Enter=execute • ?=help • q=quit"
		if m.Width < 60 {
			helpText = "i=insert • j/k=nav • Enter=run • ?=help • q=quit"
		}
		b.WriteString(m.Styles.HelpText.Render(normalizeMultiline(helpText)))
	} else if m.Mode == InsertMode {
		b.WriteString(m.Styles.HelpText.Render("INSERT mode • type to edit • Esc=normal mode"))
	}

	return m.Styles.Container.Render(b.String())
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

	// Calculate available height for results
	availableHeight := m.Height - 8 // Reserve space for header, status, and help
	if m.IsSearching {
		availableHeight -= 2
	}
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Build results list
	var content strings.Builder
	start := m.Viewport.YOffset
	end := start + availableHeight
	if end > len(m.FilteredResults) {
		end = len(m.FilteredResults)
	}
	if start < 0 {
		start = 0
	}

	// Calculate max line width
	maxLineWidth := m.Width - 6 // Account for margins and cursor indicator
	if maxLineWidth < 40 {
		maxLineWidth = 40
	}

	for i := start; i < end; i++ {
		if i >= len(m.FilteredResults) {
			break
		}

		asset := m.FilteredResults[i]
		line := fmt.Sprintf("[%s] %s (from %s)", asset.Type, normalizeString(asset.Name), asset.Source)

		// Truncate line if too long
		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}
		line += "\n"

		if i == m.Cursor {
			content.WriteString(m.Styles.ResultSelected.Render("> " + line))
		} else {
			content.WriteString(m.Styles.ResultItem.Render("  " + line))
		}
	}

	// Update viewport content
	m.Viewport.Width = m.Width - 4
	m.Viewport.Height = availableHeight
	m.Viewport.SetContent(content.String())
	b.WriteString(m.Viewport.View())

	// Status bar
	b.WriteString("\n")
	status := fmt.Sprintf("%d/%d results • Cursor: %d", len(m.FilteredResults), len(m.Results), m.Cursor+1)
	if m.StatusMessage != "" && !m.IsSearching {
		status = m.StatusMessage
	}
	if len(status) > m.Width-4 {
		status = status[:m.Width-7] + "..."
	}
	b.WriteString(m.Styles.StatusBar.Render(status))

	b.WriteString("\n")
	helpText := "j/↓/k/↑ to navigate • g/G for top/bottom • / to search • y to copy • b/esc back • ? help • q quit"
	if m.Width < 80 {
		helpText = "j/k=nav • g/G=top/bot • /=search • y=copy • b=back • ?=help • q=quit"
	}
	b.WriteString(m.Styles.HelpText.Render(helpText))

	return m.Styles.Container.Render(b.String())
}

// viewHelpScreen renders the help screen
func (m Model) viewHelpScreen() string {
	var b strings.Builder

	b.WriteString(m.Styles.Title.Render("Help"))
	b.WriteString("\n\n")

	helpText := "Query Builder - Vim Modes:\n\n"

	if m.Width >= 80 {
		helpText += `NORMAL Mode (default):
  i, I                     Enter INSERT mode to edit field
  j, ↓, k, ↑              Navigate between fields
  Tab, Shift+Tab          Alternative navigation
  Enter                   Execute the query
  Ctrl+L                  Clear all fields
  ? or h                  Show this help
  q or Ctrl+C             Quit

INSERT Mode (when editing):
  Type characters         Enter text into the field
  Esc                     Return to NORMAL mode
  Ctrl+C                  Quit

Results Screen:
  j/↓ or k/↑              Move cursor down/up
  g                       Go to first result
  G                       Go to last result
  Ctrl+D                  Half page down
  Ctrl+U                  Half page up
  /                       Search in results
  n/N                     Next/previous search result
  y                       Copy current item name
  b or Esc                Back to query builder
  ? or h                  Show this help
  q or Ctrl+C             Quit`
	} else {
		helpText += `NORMAL Mode:
  i=insert • j/k=nav • Enter=exec • Ctrl+L=clear • ?=help • q=quit

INSERT Mode:
  Type=edit • Esc=normal • Ctrl+C=quit

Results Screen:
  j/k=nav • g/G=top/bot • /=search • n/N=next/prev • y=copy
  b/Esc=back • ?=help • q=quit`
	}

	helpText += `

Query Syntax:
  -cmd: index, list, search, export
  -map: Comma-separated map names (zm_tomb, zm_prison)
  -type: weapon, perk, xmodel, material, image
  -pattern: Comma-separated with AND logic (! to exclude)
  -format: plain, json, csv, gsc
  -output: File path or leave empty for stdout
`

	b.WriteString(m.Styles.HelpText.Render(helpText))
	b.WriteString("\n")
	b.WriteString(m.Styles.StatusBar.Render("Press q, esc, or ?/h to return"))

	return m.Styles.Container.Render(b.String())
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

// ExportCompleteMsg is sent when export completes
type ExportCompleteMsg struct {
	Filename string
	Count    int
	Error    error
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

		switch m.Query.Cmd {
		case "list", "search":
			// Execute the query
			return m.runQuery()
		case "export":
			// Execute export
			return m.runExport()
		default:
			return LoadCompleteMsg{
				Error: fmt.Errorf("command '%s' not supported in TUI (use list, search, or export)", m.Query.Cmd),
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

// runExport runs the export and returns completion message
func (m Model) runExport() tea.Msg {
	results, err := ExecuteQuery(m.ZonePath, m.Query, m.UseCache)
	if err != nil {
		return ExportCompleteMsg{
			Error: err,
		}
	}

	// Determine output file
	outputFile := m.Query.Output
	if outputFile == "" {
		outputFile = "export.txt"
	}

	// Export the results
	count, err := ExportToFile(results, m.Query.Format, outputFile)
	return ExportCompleteMsg{
		Filename: outputFile,
		Count:    count,
		Error:    err,
	}
}
