package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/maxvanasten/t6-asset-browser/internal/fastfile"
	"github.com/maxvanasten/t6-asset-browser/pkg/t6assets"
)

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

// Model represents the TUI state
type Model struct {
	Screen          Screen
	Mode            Mode
	Registry        *t6assets.Registry
	ZonePath        string
	UseCache        bool
	Query           QueryConfig
	ActiveField     QueryField
	Fields          []QueryField
	FieldInputs     map[QueryField]*textinput.Model
	Results         []*t6assets.Asset
	FilteredResults []*t6assets.Asset
	Viewport        viewport.Model
	Cursor          int
	SearchInput     *textinput.Model
	IsSearching     bool
	StatusMessage   string
	IsLoading       bool
	Error           error
	Width           int
	Height          int
}

// NewModel creates a new TUI model
func NewModel(zonePath string, useCache bool) Model {
	fields := []QueryField{CmdField, MapField, TypeField, PatternField, FormatField, OutputField}
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

	searchInput := textinput.New()
	searchInput.Placeholder = "Search in results..."
	searchInput.Width = 40

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
		m.Viewport.Width = msg.Width - 2
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

func (m Model) updateQueryBuilder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.Mode == InsertMode {
		switch msg.String() {
		case "esc":
			m.Mode = NormalMode
			m.StatusMessage = "NORMAL mode - press 'i' to edit, '?' for help"
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		default:
			activeInput := m.FieldInputs[m.ActiveField]
			newInput, cmd := activeInput.Update(msg)
			*m.FieldInputs[m.ActiveField] = newInput
			return m, cmd
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "i", "I":
		m.Mode = InsertMode
		m.StatusMessage = "INSERT mode - type to edit, Esc for Normal mode"
		m.FieldInputs[m.ActiveField].Focus()
		return m, nil
	case "j", "down":
		m.ActiveField = m.Fields[(int(m.ActiveField)+1)%len(m.Fields)]
		return m, nil
	case "k", "up":
		idx := int(m.ActiveField) - 1
		if idx < 0 {
			idx = len(m.Fields) - 1
		}
		m.ActiveField = m.Fields[idx]
		return m, nil
	case "tab":
		m.ActiveField = m.Fields[(int(m.ActiveField)+1)%len(m.Fields)]
		return m, nil
	case "shift+tab":
		idx := int(m.ActiveField) - 1
		if idx < 0 {
			idx = len(m.Fields) - 1
		}
		m.ActiveField = m.Fields[idx]
		return m, nil
	case "enter":
		m.IsLoading = true
		m.StatusMessage = "Loading..."
		return m, m.executeQuery()
	case "?", "h":
		m.Screen = HelpScreen
		return m, nil
	case "ctrl+l":
		for _, field := range m.Fields {
			m.FieldInputs[field].SetValue("")
		}
		m.FieldInputs[CmdField].SetValue("list")
		m.FieldInputs[FormatField].SetValue("plain")
		return m, nil
	}

	return m, nil
}

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
		m.Screen = QueryBuilderScreen
		m.Error = nil
		return m, nil
	case "j", "down":
		if m.Cursor < len(m.FilteredResults)-1 {
			m.Cursor++
			m.updateViewport()
		}
	case "k", "up":
		if m.Cursor > 0 {
			m.Cursor--
			m.updateViewport()
		}
	case "g":
		m.Cursor = 0
		m.updateViewport()
	case "G":
		m.Cursor = len(m.FilteredResults) - 1
		if m.Cursor < 0 {
			m.Cursor = 0
		}
		m.updateViewport()
	case "ctrl+d":
		m.Cursor += m.Viewport.Height / 2
		if m.Cursor >= len(m.FilteredResults) {
			m.Cursor = len(m.FilteredResults) - 1
			if m.Cursor < 0 {
				m.Cursor = 0
			}
		}
		m.updateViewport()
	case "ctrl+u":
		m.Cursor -= m.Viewport.Height / 2
		if m.Cursor < 0 {
			m.Cursor = 0
		}
		m.updateViewport()
	case "/":
		m.IsSearching = true
		return m, m.SearchInput.Focus()
	case "n":
		m.findNext()
	case "N":
		m.findPrevious()
	case "y":
		if m.Cursor < len(m.FilteredResults) {
			asset := m.FilteredResults[m.Cursor]
			m.StatusMessage = fmt.Sprintf("Copied: %s", asset.Name)
		}
	case "?", "h":
		m.Screen = HelpScreen
		return m, nil
	}

	return m, nil
}

func (m Model) updateHelpScreen(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "?", "h":
		if len(m.Results) > 0 {
			m.Screen = ResultsScreen
		} else {
			m.Screen = QueryBuilderScreen
		}
		return m, nil
	}
	return m, nil
}

func (m Model) viewQueryBuilder() string {
	if m.Width < 40 || m.Height < 15 {
		return "Terminal too small. Please resize to at least 40x15."
	}

	availableWidth := m.Width - 15
	if availableWidth < 10 {
		availableWidth = 10
	}
	if availableWidth > 50 {
		availableWidth = 50
	}

	for _, field := range m.Fields {
		m.FieldInputs[field].Width = availableWidth
	}

	var b strings.Builder

	modeStr := ""
	if m.Mode == NormalMode {
		modeStr = " [NORMAL]"
	} else {
		modeStr = " [INSERT]"
	}

	b.WriteString("T6 Asset Browser" + modeStr)
	b.WriteString("\n")
	b.WriteString("Configure your query below")
	b.WriteString("\n\n")

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

		if field == m.ActiveField {
			b.WriteString("> ")
		} else {
			b.WriteString("  ")
		}

		b.WriteString(label + ": ")

		if field == m.ActiveField {
			b.WriteString(input.View())
		} else {
			displayValue := input.Value()
			if displayValue == "" {
				displayValue = input.Placeholder
			}
			b.WriteString(displayValue)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if m.IsLoading {
		loadingMsg := "Loading..."
		if m.Query.Cmd == "export" {
			loadingMsg = "Exporting assets..."
		}
		b.WriteString(loadingMsg)
		b.WriteString("\n")
	} else if m.Error != nil {
		b.WriteString(fmt.Sprintf("Error: %v", m.Error))
		b.WriteString("\n")
	} else {
		b.WriteString(m.StatusMessage)
	}

	b.WriteString("\n")
	if m.Mode == NormalMode && !m.IsLoading {
		b.WriteString("i=insert | j/k=navigate | Enter=execute | ?=help | q=quit")
	} else if m.Mode == InsertMode {
		b.WriteString("INSERT mode | type to edit | Esc=normal mode")
	}

	return b.String()
}

func (m Model) viewResultsScreen() string {
	if m.Width < 40 || m.Height < 10 {
		return "Terminal too small. Please resize to at least 40x10."
	}

	var b strings.Builder

	b.WriteString("Results")
	b.WriteString("\n")

	if m.IsSearching {
		b.WriteString("/" + m.SearchInput.View())
		b.WriteString("\n")
	}

	// Calculate visible lines
	availableHeight := m.Height - 8
	if m.IsSearching {
		availableHeight -= 2
	}
	if availableHeight < 5 {
		availableHeight = 5
	}

	maxLineWidth := m.Width - 4
	if maxLineWidth < 30 {
		maxLineWidth = 30
	}

	// Calculate which items to show
	start := m.Viewport.YOffset
	end := start + availableHeight
	if end > len(m.FilteredResults) {
		end = len(m.FilteredResults)
	}

	// Build visible content
	for i := start; i < end; i++ {
		if i >= len(m.FilteredResults) {
			break
		}

		asset := m.FilteredResults[i]
		line := fmt.Sprintf("[%s] %s (from %s)", asset.Type, asset.Name, asset.Source)

		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}

		if i == m.Cursor {
			b.WriteString("> " + line + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	b.WriteString("\n")
	status := fmt.Sprintf("%d/%d results | Cursor: %d", len(m.FilteredResults), len(m.Results), m.Cursor+1)
	if m.StatusMessage != "" && !m.IsSearching {
		status = m.StatusMessage
	}
	b.WriteString(status)

	b.WriteString("\n")
	b.WriteString("j/k=navigate | g/G=top/bottom | /=search | y=copy | b/esc=back | ?=help | q=quit")

	return b.String()
}

func (m Model) viewHelpScreen() string {
	if m.Width < 30 || m.Height < 10 {
		return "Terminal too small. Please resize."
	}

	var b strings.Builder

	b.WriteString("Help")
	b.WriteString("\n\n")

	var helpText string
	if m.Width >= 80 {
		helpText = `Query Builder - Vim Modes:

NORMAL Mode (default):
  i, I                     Enter INSERT mode to edit field
  j, down, k, up          Navigate between fields
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
  j/down or k/up          Move cursor down/up
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
	} else if m.Width >= 50 {
		helpText = `Query Builder:
  i=insert | j/k=nav | Enter=exec | Ctrl+L=clear | ?=help | q=quit

INSERT Mode:
  Type=edit | Esc=normal | Ctrl+C=quit

Results Screen:
  j/k=nav | g/G=top/bot | /=search | n/N=next/prev | y=copy
  b/Esc=back | ?=help | q=quit`
	} else {
		helpText = `Keys:
  i=edit  j/k=move  Enter=go  ?=help  q=quit
  Esc=normal  Ctrl+C=quit`
	}

	b.WriteString(helpText)
	b.WriteString("\n\n")
	b.WriteString("Press q, esc, or ?/h to return")

	return b.String()
}

func (m *Model) updateViewport() {
	if m.Cursor < m.Viewport.YOffset {
		m.Viewport.YOffset = m.Cursor
	} else if m.Cursor >= m.Viewport.YOffset+m.Viewport.Height {
		m.Viewport.YOffset = m.Cursor - m.Viewport.Height + 1
	}
}

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

type LoadCompleteMsg struct {
	Assets []*t6assets.Asset
	Error  error
}

type ExportCompleteMsg struct {
	Filename string
	Count    int
	Error    error
}

func (m Model) executeQuery() tea.Cmd {
	return func() tea.Msg {
		m.Query.Cmd = m.FieldInputs[CmdField].Value()
		m.Query.Map = m.FieldInputs[MapField].Value()
		m.Query.Type = m.FieldInputs[TypeField].Value()
		m.Query.Pattern = m.FieldInputs[PatternField].Value()
		m.Query.Format = m.FieldInputs[FormatField].Value()
		m.Query.Output = m.FieldInputs[OutputField].Value()

		switch m.Query.Cmd {
		case "list", "search":
			return m.runQuery()
		case "export":
			return m.runExport()
		default:
			return LoadCompleteMsg{
				Error: fmt.Errorf("command '%s' not supported in TUI (use list, search, or export)", m.Query.Cmd),
			}
		}
	}
}

func (m Model) runQuery() tea.Msg {
	results, err := ExecuteQuery(m.ZonePath, m.Query, m.UseCache)
	return LoadCompleteMsg{
		Assets: results,
		Error:  err,
	}
}

func (m Model) runExport() tea.Msg {
	results, err := ExecuteQuery(m.ZonePath, m.Query, m.UseCache)
	if err != nil {
		return ExportCompleteMsg{
			Error: err,
		}
	}

	outputFile := m.Query.Output
	if outputFile == "" {
		outputFile = "export.txt"
	}

	count, err := ExportToFile(results, m.Query.Format, outputFile)
	return ExportCompleteMsg{
		Filename: outputFile,
		Count:    count,
		Error:    err,
	}
}

func ExecuteQuery(zonePath string, query QueryConfig, useCache bool) ([]*t6assets.Asset, error) {
	registry := t6assets.NewRegistry()

	var filesToProcess []string

	if query.Map != "" {
		allFiles, _ := filepath.Glob(filepath.Join(zonePath, "*.ff"))
		mapList := strings.Split(query.Map, ",")
		for _, ffPath := range allFiles {
			_, fileName := filepath.Split(ffPath)
			for _, m := range mapList {
				m = strings.TrimSpace(m)
				if m != "" {
					searchTerm := m
					if strings.HasSuffix(searchTerm, ".ff") {
						searchTerm = searchTerm[:len(searchTerm)-3]
					}
					if strings.Contains(fileName, searchTerm) {
						filesToProcess = append(filesToProcess, ffPath)
						break
					}
				}
			}
		}
		if len(filesToProcess) > 0 {
			fmt.Fprintf(os.Stderr, "Processing %d files matching '%s'\n", len(filesToProcess), query.Map)
		}
	}

	if len(filesToProcess) == 0 {
		filesToProcess, _ = filepath.Glob(filepath.Join(zonePath, "*.ff"))
		fmt.Fprintf(os.Stderr, "Processing all %d files\n", len(filesToProcess))
	}

	if err := indexFilesParallel(filesToProcess, registry, useCache); err != nil {
		return nil, fmt.Errorf("failed to index FastFiles: %w", err)
	}

	var results []*t6assets.Asset
	switch query.Cmd {
	case "list":
		results = filterAssets(registry, query)
	case "search":
		results = filterAssets(registry, query)
	default:
		return nil, fmt.Errorf("unsupported command: %s", query.Cmd)
	}

	return results, nil
}

func filterAssets(registry *t6assets.Registry, query QueryConfig) []*t6assets.Asset {
	var results []*t6assets.Asset

	for _, asset := range registry.Assets {
		if query.Type != "" {
			typeList := strings.Split(query.Type, ",")
			validTypes := make(map[t6assets.AssetType]bool)
			for _, t := range typeList {
				t = strings.TrimSpace(t)
				if t != "" {
					validTypes[parseAssetType(t)] = true
				}
			}
			if !validTypes[asset.Type] {
				continue
			}
		}

		if query.Map != "" {
			mapList := strings.Split(query.Map, ",")
			matched := false
			for _, m := range mapList {
				m = strings.TrimSpace(m)
				if m != "" {
					searchTerm := m
					if strings.HasSuffix(searchTerm, ".ff") {
						searchTerm = searchTerm[:len(searchTerm)-3]
					}
					if strings.Contains(asset.Source, searchTerm) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		if query.Pattern != "" {
			include, exclude := parsePatterns(query.Pattern)
			if !matchesPatterns(asset.Name, include, exclude, query.UseWildcard, query.IgnoreCase) {
				continue
			}
		}

		results = append(results, asset)
	}

	return results
}

func indexFilesParallel(ffFiles []string, registry *t6assets.Registry, useCache bool) error {
	if len(ffFiles) == 0 {
		return fmt.Errorf("no files to process")
	}

	var rawCache *fastfile.Cache
	if useCache {
		rawCache, _ = fastfile.NewCache()
	}

	oat := fastfile.NewOATIntegration()
	if oat.IsAvailable() {
		fmt.Fprintf(os.Stderr, "Using OpenAssetTools for decryption\n")
	}

	totalFiles := len(ffFiles)
	startTime := time.Now()

	numWorkers := 4
	if totalFiles < numWorkers {
		numWorkers = totalFiles
	}

	fileChan := make(chan string, totalFiles)
	for _, ffPath := range ffFiles {
		fileChan <- ffPath
	}
	close(fileChan)

	type fileResult struct {
		fileName string
		assets   []*t6assets.Asset
		err      error
	}
	resultChan := make(chan fileResult, totalFiles)

	var processedCount int
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for ffPath := range fileChan {
				_, fileName := filepath.Split(ffPath)

				assets, err := processSingleFile(ffPath, fileName, oat, rawCache, useCache)

				mu.Lock()
				processedCount++
				current := processedCount
				mu.Unlock()

				if err != nil {
					fmt.Fprintf(os.Stderr, "[%d/%d] Error processing %s: %v\n",
						current, totalFiles, fileName, err)
				} else {
					fmt.Fprintf(os.Stderr, "[%d/%d] Indexed: %s (%d assets)\n",
						current, totalFiles, fileName, len(assets))
				}

				resultChan <- fileResult{
					fileName: fileName,
					assets:   assets,
					err:      err,
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for result := range resultChan {
		if result.err == nil {
			for _, asset := range result.assets {
				registry.Add(asset)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "Total: %d files processed, %d assets indexed in %v\n",
		totalFiles, len(registry.Assets), time.Since(startTime))

	return nil
}

func processSingleFile(ffPath, fileName string, oat *fastfile.OATIntegration, rawCache *fastfile.Cache, useCache bool) ([]*t6assets.Asset, error) {
	var assets []*t6assets.Asset

	if oat.IsAvailable() {
		assetNames, assetTypes, err := oat.ExtractAndParseZone(ffPath)
		if err == nil && len(assetNames) > 0 {
			for _, name := range assetNames {
				assetType := parseOATAssetType(assetTypes[name])
				assets = append(assets, &t6assets.Asset{
					Name:   name,
					Type:   assetType,
					Source: fileName,
				})
			}
			return assets, nil
		}
	}

	data, err := os.ReadFile(ffPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	reader := fastfile.NewReader()
	zoneData, err := reader.Read(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	if useCache && rawCache != nil {
		rawCache.WriteCache(ffPath, zoneData)
	}

	tempRegistry := t6assets.NewRegistry()
	parser := fastfile.NewParser(tempRegistry)

	if err := parser.Parse(zoneData, fileName); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	for _, asset := range tempRegistry.Assets {
		assets = append(assets, asset)
	}

	return assets, nil
}

func ExportToFile(assets []*t6assets.Asset, format string, filename string) (int, error) {
	file, err := os.Create(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	switch format {
	case "plain", "":
		for _, a := range assets {
			fmt.Fprintln(file, a.Name)
		}
	case "json":
		fmt.Fprintln(file, "[")
		for i, a := range assets {
			comma := ","
			if i == len(assets)-1 {
				comma = ""
			}
			fmt.Fprintf(file, "  {\"name\": \"%s\", \"type\": \"%s\", \"source\": \"%s\"}%s\n",
				a.Name, a.Type, a.Source, comma)
		}
		fmt.Fprintln(file, "]")
	case "csv":
		fmt.Fprintln(file, "name,type,source")
		for _, a := range assets {
			fmt.Fprintf(file, "%s,%s,%s\n", a.Name, a.Type, a.Source)
		}
	case "gsc":
		fmt.Fprintln(file, "array(")
		for _, a := range assets {
			fmt.Fprintf(file, "\t\"%s\",\n", a.Name)
		}
		fmt.Fprintln(file, ")")
	default:
		return 0, fmt.Errorf("unknown format: %s", format)
	}

	return len(assets), nil
}

func parseAssetType(s string) t6assets.AssetType {
	switch s {
	case "weapon":
		return t6assets.AssetTypeWeapon
	case "xmodel":
		return t6assets.AssetTypeXModel
	case "perk":
		return t6assets.AssetTypePerk
	case "material":
		return t6assets.AssetTypeMaterial
	case "image":
		return t6assets.AssetTypeImage
	default:
		return t6assets.AssetTypeUnknown
	}
}

func parseOATAssetType(oatType string) t6assets.AssetType {
	switch oatType {
	case "weapon":
		return t6assets.AssetTypeWeapon
	case "xmodel":
		return t6assets.AssetTypeXModel
	case "material":
		return t6assets.AssetTypeMaterial
	case "image":
		return t6assets.AssetTypeImage
	case "fx":
		return t6assets.AssetTypeFX
	case "perk":
		return t6assets.AssetTypePerk
	default:
		return t6assets.AssetTypeUnknown
	}
}

func parsePatterns(pattern string) (include []string, exclude []string) {
	if pattern == "" {
		return nil, nil
	}

	parts := strings.Split(pattern, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "!") {
			exclude = append(exclude, part[1:])
		} else {
			include = append(include, part)
		}
	}
	return include, exclude
}

func matchesPatterns(str string, include []string, exclude []string, useWildcard bool, ignoreCase bool) bool {
	for _, pattern := range include {
		var matched bool
		if useWildcard {
			if ignoreCase {
				matched = wildcardMatch(strings.ToLower(str), strings.ToLower(pattern))
			} else {
				matched = wildcardMatch(str, pattern)
			}
		} else if ignoreCase {
			matched = containsIgnoreCase(str, pattern)
		} else {
			matched = strings.Contains(str, pattern)
		}
		if !matched {
			return false
		}
	}

	for _, pattern := range exclude {
		var matched bool
		if useWildcard {
			if ignoreCase {
				matched = wildcardMatch(strings.ToLower(str), strings.ToLower(pattern))
			} else {
				matched = wildcardMatch(str, pattern)
			}
		} else if ignoreCase {
			matched = containsIgnoreCase(str, pattern)
		} else {
			matched = strings.Contains(str, pattern)
		}
		if matched {
			return false
		}
	}

	return true
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	lowerS := strings.ToLower(s)
	lowerSubstr := strings.ToLower(substr)
	return strings.Contains(lowerS, lowerSubstr)
}

func wildcardMatch(str, pattern string) bool {
	if len(pattern) == 0 {
		return len(str) == 0
	}

	if len(str) == 0 {
		for _, p := range pattern {
			if p != '*' {
				return false
			}
		}
		return true
	}

	if pattern[0] == '*' {
		return wildcardMatch(str, pattern[1:]) || wildcardMatch(str[1:], pattern)
	} else if pattern[0] == '?' || pattern[0] == str[0] {
		return wildcardMatch(str[1:], pattern[1:])
	}

	return false
}
