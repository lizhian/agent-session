package picker

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lizhian/agent-session/internal/provider"
	"github.com/lizhian/agent-session/internal/render"
	"github.com/lizhian/agent-session/internal/session"
	"github.com/lizhian/agent-session/internal/skills"
)

// View represents the current screen in the picker state machine.
type View int

const (
	ViewSessions View = iota
	ViewPreview
	ViewWorkspaces
	ViewConfigurations
	ViewConfigurationItems
	ViewConfigurationSubitems
	ViewManagerHub
	ViewSkillsManager
	ViewSkillsInstallInput
	ViewSkillsGlobal
	ViewSkillsProject
	ViewSkillSourceDetail
	ViewSkillSourceRemoveConfirm
)

type skillScope int

const (
	skillScopeGlobal skillScope = iota
	skillScopeProject
)

type managerItem struct {
	Label string
	Kind  string
}

type skillManagerItem struct {
	Label      string
	Kind       string
	SourceSafe string
	Source     string
}

type skillSelectionItem struct {
	Label       string
	Description string
	Kind        string
	Source      string
	SourceSafe  string
	SkillName   string
	LinkName    string
	Selected    bool
}

type skillSourceDetailItem struct {
	Label       string
	Description string
	Kind        string
	SourceSafe  string
	Source      string
	SkillName   string
}

// Model is the bubbletea Model for the interactive session picker.
type Model struct {
	provider   provider.Provider
	sessions   []provider.Session
	workspaces []provider.Workspace
	view       View

	// Navigation state.
	sessionSelectedIndex     int
	workspaceSelectedIndex   int
	managerSelectedIndex     int
	skillsSelectedIndex      int
	skillDetailSelectedIndex int
	skillRemoveSelectedIndex int

	// Search queries.
	sessionQuery    string
	workspaceQuery  string
	skillsInput     string
	skillsCursor    int
	skillsTextInput textinput.Model

	// Permission mode.
	permissionMode string

	// Current working directory.
	cwd     string
	cwdFile string

	// Configuration state.
	config configurationWorkflow

	// Skills manager state.
	skillsStore             *skills.Store
	skillsCatalog           skills.Catalog
	skillsManagerStatus     string
	skillSelectionStatus    string
	skillDetailStatus       string
	skillRemoveStatus       string
	managerItems            []managerItem
	skillManagerItems       []skillManagerItem
	skillSelectionItems     []skillSelectionItem
	skillSelectionScope     skillScope
	selectedSkillSourceSafe string
	skillSourceDetailItems  []skillSourceDetailItem
	skillRemoveImpact       skills.RemoveSourceImpact

	// Preview state.
	previewTranscript []provider.TranscriptMessage
	previewError      string
	previewScroll     int
	previewAutoBottom bool

	// Terminal dimensions.
	width  int
	height int

	// Color output.
	useColor bool

	// Result.
	result   *provider.PickResult
	quitting bool
}

// NewModel creates a new picker model.
func NewModel(p provider.Provider, sessions []provider.Session, cwd, permissionMode string, width, height int, useColor bool) Model {
	model := Model{
		provider:             p,
		sessions:             sessions,
		view:                 ViewSessions,
		sessionSelectedIndex: 1,
		permissionMode:       session.NormalizePermissionMode(permissionMode, p.PermissionModes()),
		cwd:                  cwd,
		cwdFile:              os.Getenv("AGENT_SESSION_CWD_FILE"),
		width:                width,
		height:               height,
		useColor:             useColor,
		config:               newConfigurationWorkflow(p),
		skillsStore:          skills.NewStore(""),
	}
	model.initSkillsTextInput()
	return model
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.view == ViewSkillsInstallInput {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "esc", "enter", "return", "up", "k", "down", "j":
				// Let the picker-level handlers manage navigation and submit.
			default:
				var cmd tea.Cmd
				m.skillsTextInput, cmd = m.skillsTextInput.Update(msg)
				m.syncSkillsInputFromTextInput()
				return m, cmd
			}
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			m.result = nil
			return m, tea.Quit
		}

		switch msg.String() {
		case "esc":
			return m.handleEscape()
		case "enter", "return":
			return m.handleEnter()
		case " ":
			return m.handleSpace()
		case "tab":
			return m.handleTab()
		case "up", "k":
			return m.handleUp()
		case "down", "j":
			return m.handleDown()
		case "left", "h":
			return m.handleLeft()
		case "right", "l":
			return m.handleRight()
		case "backspace":
			return m.handleBackspace()
		case "delete":
			return m.handleBackspace()
		default:
			// Forward any printable text, including pasted strings and IME commits.
			if len(msg.Runes) > 0 {
				return m.handleText(string(msg.Runes))
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.view {
	case ViewSessions:
		return m.renderSessions()
	case ViewPreview:
		return m.renderPreview()
	case ViewWorkspaces:
		return m.renderWorkspaces()
	case ViewConfigurations:
		return m.renderConfigurations()
	case ViewConfigurationItems:
		return m.renderConfigurationItems()
	case ViewConfigurationSubitems:
		return m.renderConfigurationSubitems()
	case ViewManagerHub:
		return m.renderManagerHub()
	case ViewSkillsManager:
		return m.renderSkillsManager()
	case ViewSkillsInstallInput:
		return m.renderSkillsInstallInput()
	case ViewSkillsGlobal, ViewSkillsProject:
		return m.renderSkillSelectionView()
	case ViewSkillSourceDetail:
		return m.renderSkillSourceDetail()
	case ViewSkillSourceRemoveConfirm:
		return m.renderSkillSourceRemoveConfirm()
	}
	return ""
}

// Result returns the picker result after the program exits.
func (m Model) Result() *provider.PickResult {
	return m.result
}

func (m *Model) initSkillsTextInput() {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "owner/repo"
	input.CharLimit = 0
	input.Focus()
	input.SetValue(m.skillsInput)
	input.SetCursor(len([]rune(m.skillsInput)))
	m.skillsTextInput = input
	m.syncSkillsInputFromTextInput()
}

func (m *Model) syncSkillsInputFromTextInput() {
	m.skillsInput = m.skillsTextInput.Value()
	m.skillsCursor = m.skillsTextInput.Position()
}

func (m *Model) syncTextInputFromSkillsInput() {
	m.skillsTextInput.SetValue(m.skillsInput)
	m.skillsTextInput.SetCursor(m.skillsCursor)
	m.syncSkillsInputFromTextInput()
}

// --- Key handlers ---

func scrollLinesPerWheel() int {
	value := strings.TrimSpace(os.Getenv("AGENT_SESSION_SCROLL_LINES"))
	if value == "" {
		return 3
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 3
	}
	if n < 1 {
		return 1
	}
	if n > 20 {
		return 20
	}
	return n
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.view != ViewPreview {
		return m, nil
	}
	switch msg.Type {
	case tea.MouseWheelUp:
		m.previewAutoBottom = false
		m.previewScroll -= scrollLinesPerWheel()
		if m.previewScroll < 0 {
			m.previewScroll = 0
		}
	case tea.MouseWheelDown:
		m.previewAutoBottom = false
		m.previewScroll += scrollLinesPerWheel()
	}
	return m, nil
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewPreview:
		m.view = ViewSessions
		m.previewTranscript = nil
		m.previewError = ""
		m.previewScroll = 0
		m.previewAutoBottom = false
		return m, nil
	case ViewConfigurationSubitems:
		return m.cancelConfigurationSubitems()
	case ViewConfigurationItems:
		m.view = ViewConfigurations
		m.config.cancelItems()
		return m, nil
	case ViewConfigurations:
		m.view = ViewWorkspaces
		return m, nil
	case ViewWorkspaces:
		m.view = ViewSessions
		return m, nil
	case ViewManagerHub:
		m.view = ViewSessions
		return m, nil
	case ViewSkillsManager:
		m.view = ViewManagerHub
		return m, nil
	case ViewSkillsInstallInput:
		m.view = ViewSkillsManager
		m.skillsInput = ""
		m.skillsCursor = 0
		m.syncTextInputFromSkillsInput()
		return m, nil
	case ViewSkillsGlobal, ViewSkillsProject:
		m.view = ViewSkillsManager
		m.skillSelectionItems = nil
		m.skillSelectionStatus = ""
		return m, nil
	case ViewSkillSourceDetail:
		m.view = ViewSkillsManager
		m.skillSourceDetailItems = nil
		m.skillDetailStatus = ""
		return m, nil
	case ViewSkillSourceRemoveConfirm:
		m.view = ViewSkillSourceDetail
		m.skillRemoveStatus = ""
		return m, nil
	default:
		m.quitting = true
		m.result = nil
		return m, tea.Quit
	}
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewSessions:
		return m.selectSession()
	case ViewPreview:
		// Enter in preview does nothing (space returns to sessions).
		return m, nil
	case ViewWorkspaces:
		return m.selectWorkspace()
	case ViewConfigurations:
		return m.selectConfiguration()
	case ViewConfigurationItems:
		return m.selectConfigurationItem()
	case ViewConfigurationSubitems:
		return m.selectConfigurationSubitems()
	case ViewManagerHub:
		return m.selectManagerItem()
	case ViewSkillsManager:
		return m.selectSkillsManagerItem()
	case ViewSkillsInstallInput:
		return m.installSkillSource()
	case ViewSkillsGlobal, ViewSkillsProject:
		return m.saveSkillSelection()
	case ViewSkillSourceDetail:
		return m.selectSkillSourceDetailItem()
	case ViewSkillSourceRemoveConfirm:
		return m.selectSkillSourceRemoveConfirmItem()
	}
	return m, nil
}

func (m Model) handleSpace() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewSessions:
		// Enter terminal preview mode for selected session.
		items := m.currentSessionItems()
		idx := render.ClampSelectedIndex(m.sessionSelectedIndex, len(items))
		if idx < len(items) && items[idx].Type == "session" && items[idx].Session != nil {
			s := *items[idx].Session
			m.previewTranscript = m.provider.LoadSessionTranscript(s, provider.Context{Cwd: m.cwd})
			m.previewError = ""
			m.previewScroll = 0
			m.previewAutoBottom = true
			m.view = ViewPreview
			return m, nil
		}
		return m, nil
	case ViewPreview:
		m.view = ViewSessions
		m.previewTranscript = nil
		m.previewError = ""
		m.previewScroll = 0
		m.previewAutoBottom = false
		return m, nil
	case ViewConfigurationSubitems:
		m.config.toggleSubitem()
		return m, nil
	case ViewSkillsGlobal, ViewSkillsProject:
		idx := render.ClampSelectedIndex(m.skillsSelectedIndex, len(m.skillSelectionItems))
		if idx < len(m.skillSelectionItems) && m.skillSelectionItems[idx].Kind == "skill" {
			m.skillSelectionItems[idx].Selected = !m.skillSelectionItems[idx].Selected
			for m.skillsSelectedIndex+1 < len(m.skillSelectionItems) && m.skillSelectionItems[m.skillsSelectedIndex+1].Kind == "header" {
				m.skillsSelectedIndex++
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleTab() (tea.Model, tea.Cmd) {
	if m.view != ViewSessions {
		return m, nil
	}
	modes := m.provider.PermissionModes()
	m.permissionMode = session.NextPermissionMode(m.permissionMode, modes)
	ctx := provider.Context{Cwd: m.cwd, DataHome: m.provider.DefaultHome()}
	_ = m.provider.SavePermissionMode(m.permissionMode, ctx)
	return m, nil
}

func (m Model) handleUp() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewSessions:
		items := m.currentSessionItems()
		if m.sessionSelectedIndex > 0 {
			m.sessionSelectedIndex--
		}
		_ = items // ensure computed
		return m, nil
	case ViewPreview:
		m.previewAutoBottom = false
		if m.previewScroll > 0 {
			m.previewScroll--
		}
		return m, nil
	case ViewWorkspaces:
		if m.workspaceSelectedIndex > 0 {
			m.workspaceSelectedIndex--
		}
		return m, nil
	case ViewConfigurations:
		m.config.moveAction(-1)
		return m, nil
	case ViewManagerHub:
		if m.managerSelectedIndex > 0 {
			m.managerSelectedIndex--
		}
		return m, nil
	case ViewSkillsManager:
		if m.skillsSelectedIndex > 0 {
			m.skillsSelectedIndex--
		}
		return m, nil
	case ViewSkillsGlobal, ViewSkillsProject:
		if idx := previousSelectableSkillIndex(m.skillSelectionItems, m.skillsSelectedIndex); idx != m.skillsSelectedIndex {
			m.skillsSelectedIndex = idx
		}
		return m, nil
	case ViewConfigurationItems, ViewConfigurationSubitems:
		if m.view == ViewConfigurationItems {
			m.config.moveItem(-1)
		} else {
			m.config.moveSubitem(-1)
		}
		return m, nil
	case ViewSkillSourceDetail:
		if m.skillDetailSelectedIndex > 0 {
			m.skillDetailSelectedIndex--
		}
		return m, nil
	case ViewSkillSourceRemoveConfirm:
		if m.skillRemoveSelectedIndex > 0 {
			m.skillRemoveSelectedIndex--
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewSessions:
		items := m.currentSessionItems()
		m.sessionSelectedIndex = render.ClampSelectedIndex(m.sessionSelectedIndex+1, len(items))
		return m, nil
	case ViewPreview:
		m.previewAutoBottom = false
		m.previewScroll++
		return m, nil
	case ViewWorkspaces:
		items := m.currentWorkspaceItems()
		m.workspaceSelectedIndex = render.ClampSelectedIndex(m.workspaceSelectedIndex+1, len(items))
		return m, nil
	case ViewConfigurations:
		m.config.moveAction(1)
		return m, nil
	case ViewManagerHub:
		m.managerSelectedIndex = render.ClampSelectedIndex(m.managerSelectedIndex+1, len(m.currentManagerItems()))
		return m, nil
	case ViewSkillsManager:
		m.skillsSelectedIndex = render.ClampSelectedIndex(m.skillsSelectedIndex+1, len(m.currentSkillsManagerItems()))
		return m, nil
	case ViewConfigurationItems:
		m.config.moveItem(1)
		return m, nil
	case ViewConfigurationSubitems:
		m.config.moveSubitem(1)
		return m, nil
	case ViewSkillsGlobal, ViewSkillsProject:
		m.skillsSelectedIndex = nextSelectableSkillIndex(m.skillSelectionItems, m.skillsSelectedIndex)
		return m, nil
	case ViewSkillSourceDetail:
		m.skillDetailSelectedIndex = render.ClampSelectedIndex(m.skillDetailSelectedIndex+1, len(m.skillSourceDetailItems))
		return m, nil
	case ViewSkillSourceRemoveConfirm:
		m.skillRemoveSelectedIndex = render.ClampSelectedIndex(m.skillRemoveSelectedIndex+1, 2)
		return m, nil
	}
	return m, nil
}

func (m Model) handleLeft() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewWorkspaces:
		m.view = ViewSessions
		return m, nil
	case ViewSessions:
		m.view = ViewManagerHub
		m.managerSelectedIndex = 0
		m.skillsManagerStatus = ""
		return m, nil
	case ViewConfigurations:
		m.view = ViewWorkspaces
		return m, nil
	case ViewConfigurationItems:
		m.view = ViewConfigurations
		m.config.cancelItems()
		return m, nil
	case ViewConfigurationSubitems:
		return m.cancelConfigurationSubitems()
	case ViewSkillsManager:
		m.view = ViewManagerHub
		return m, nil
	case ViewSkillsInstallInput:
		if m.skillsCursor > 0 {
			m.skillsCursor--
			return m, nil
		}
		m.view = ViewSkillsManager
		return m, nil
	case ViewSkillsGlobal, ViewSkillsProject:
		m.view = ViewSkillsManager
		return m, nil
	case ViewSkillSourceDetail:
		m.view = ViewSkillsManager
		return m, nil
	case ViewSkillSourceRemoveConfirm:
		m.view = ViewSkillSourceDetail
		return m, nil
	}
	return m, nil
}

func (m Model) cancelConfigurationSubitems() (tea.Model, tea.Cmd) {
	m.view = m.config.cancelSubitems()
	return m, nil
}

func (m Model) handleRight() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewSessions:
		if m.workspaces == nil {
			ctx := provider.Context{DataHome: m.provider.DefaultHome()}
			m.workspaces = m.provider.ListWorkspaces(ctx)
		}
		m.view = ViewWorkspaces
		m.workspaceSelectedIndex = 0
		return m, nil
	case ViewWorkspaces:
		m.view = ViewConfigurations
		m.config.openConfigurations()
		return m, nil
	case ViewManagerHub:
		m.view = ViewSkillsManager
		m.skillsSelectedIndex = 0
		_ = m.refreshSkillsCatalog()
		return m, nil
	case ViewSkillsInstallInput:
		cursorMax := len([]rune(m.skillsInput))
		if m.skillsCursor < cursorMax {
			m.skillsCursor++
			return m, nil
		}
		m.view = ViewSkillsManager
		return m, nil
	}
	return m, nil
}

func (m Model) handleBackspace() (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewSessions:
		if len(m.sessionQuery) > 0 {
			m.sessionQuery = m.sessionQuery[:len(m.sessionQuery)-1]
		}
		return m, nil
	case ViewWorkspaces:
		if len(m.workspaceQuery) > 0 {
			m.workspaceQuery = m.workspaceQuery[:len(m.workspaceQuery)-1]
		}
		return m, nil
	case ViewSkillsInstallInput:
		runes := []rune(m.skillsInput)
		if m.skillsCursor > 0 && m.skillsCursor <= len(runes) {
			runes = append(runes[:m.skillsCursor-1], runes[m.skillsCursor:]...)
			m.skillsInput = string(runes)
			m.skillsCursor--
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleText(text string) (tea.Model, tea.Cmd) {
	if text == "" {
		return m, nil
	}
	switch m.view {
	case ViewSessions:
		m.sessionQuery += text
		return m, nil
	case ViewWorkspaces:
		m.workspaceQuery += text
		return m, nil
	case ViewSkillsInstallInput:
		runes := []rune(m.skillsInput)
		insert := []rune(text)
		if m.skillsCursor < 0 {
			m.skillsCursor = 0
		}
		if m.skillsCursor > len(runes) {
			m.skillsCursor = len(runes)
		}
		updated := make([]rune, 0, len(runes)+len(insert))
		updated = append(updated, runes[:m.skillsCursor]...)
		updated = append(updated, insert...)
		updated = append(updated, runes[m.skillsCursor:]...)
		m.skillsInput = string(updated)
		m.skillsCursor += len(insert)
		return m, nil
	}
	return m, nil
}

// --- Selection handlers ---

func (m Model) selectSession() (tea.Model, tea.Cmd) {
	items := m.currentSessionItems()
	idx := render.ClampSelectedIndex(m.sessionSelectedIndex, len(items))
	if idx >= len(items) {
		m.quitting = true
		m.result = nil
		return m, tea.Quit
	}
	item := items[idx]
	var pickItem provider.PickItem
	if item.Type == "session" && item.Session != nil {
		pickItem = provider.PickItem{
			Type:    item.Type,
			Session: item.Session,
		}
	} else {
		pickItem = provider.PickItem{
			Type:  item.Type,
			Label: "new",
		}
	}
	m.result = &provider.PickResult{
		Item:           pickItem,
		PermissionMode: m.permissionMode,
		Cwd:            m.cwd,
	}
	m.quitting = true
	return m, tea.Quit
}

func (m Model) writeSelectedCwd() {
	if m.cwdFile == "" || m.cwd == "" {
		return
	}
	_ = os.WriteFile(m.cwdFile, []byte(m.cwd+"\n"), 0o600)
}

func (m Model) selectWorkspace() (tea.Model, tea.Cmd) {
	items := m.currentWorkspaceItems()
	idx := render.ClampSelectedIndex(m.workspaceSelectedIndex, len(items))
	if idx < len(items) && items[idx].Type == "workspace" {
		ws := items[idx].Workspace
		m.cwd = m.provider.WorkspaceCwd(ws, m.cwd)
		m.writeSelectedCwd()
		ctx := provider.Context{Cwd: m.cwd, DataHome: m.provider.DefaultHome()}
		m.sessions = m.provider.ListSessions(ctx)
		m.view = ViewSessions
		m.sessionQuery = ""
		m.sessionSelectedIndex = 0
		m.previewTranscript = nil
		m.previewError = ""
	}
	return m, nil
}

func (m Model) selectConfiguration() (tea.Model, tea.Cmd) {
	m.view = m.config.selectAction(m.cwd)
	return m, nil
}

func (m Model) selectConfigurationItem() (tea.Model, tea.Cmd) {
	m.view = m.config.selectItem(m.cwd)
	return m, nil
}

func (m Model) selectConfigurationSubitems() (tea.Model, tea.Cmd) {
	m.view = m.config.applySubitems(m.cwd)
	return m, nil
}

func (m *Model) refreshSkillsCatalog() error {
	catalog, err := m.skillsStore.LoadCatalog()
	if err != nil {
		m.skillsManagerStatus = err.Error()
		return err
	}
	m.skillsCatalog = catalog
	m.skillManagerItems = buildSkillsManagerItems(catalog)
	return nil
}

func (m Model) selectManagerItem() (tea.Model, tea.Cmd) {
	items := m.currentManagerItems()
	idx := render.ClampSelectedIndex(m.managerSelectedIndex, len(items))
	if idx >= len(items) {
		return m, nil
	}
	item := items[idx]
	if item.Kind == "skills" {
		_ = m.refreshSkillsCatalog()
		m.view = ViewSkillsManager
		m.skillsSelectedIndex = 0
	}
	return m, nil
}

func (m Model) selectSkillsManagerItem() (tea.Model, tea.Cmd) {
	items := m.currentSkillsManagerItems()
	idx := render.ClampSelectedIndex(m.skillsSelectedIndex, len(items))
	if idx >= len(items) {
		return m, nil
	}
	item := items[idx]
	switch item.Kind {
	case "install":
		m.view = ViewSkillsInstallInput
		m.skillsInput = ""
		m.skillsCursor = 0
		m.syncTextInputFromSkillsInput()
		m.skillsTextInput.Focus()
		m.skillSelectionStatus = ""
	case "global":
		if err := m.refreshSkillsCatalog(); err != nil {
			return m, nil
		}
		m.skillSelectionScope = skillScopeGlobal
		m.skillSelectionItems = buildSkillSelectionItems(m.skillsCatalog, skillScopeGlobal, m.cwd)
		m.skillsSelectedIndex = firstSelectableSkillIndex(m.skillSelectionItems)
		m.skillSelectionStatus = ""
		m.view = ViewSkillsGlobal
	case "project":
		if err := m.refreshSkillsCatalog(); err != nil {
			return m, nil
		}
		m.skillSelectionScope = skillScopeProject
		m.skillSelectionItems = buildSkillSelectionItems(m.skillsCatalog, skillScopeProject, m.cwd)
		m.skillsSelectedIndex = firstSelectableSkillIndex(m.skillSelectionItems)
		m.skillSelectionStatus = ""
		m.view = ViewSkillsProject
	case "source":
		if err := m.refreshSkillsCatalog(); err != nil {
			return m, nil
		}
		m.selectedSkillSourceSafe = item.SourceSafe
		m.skillSourceDetailItems = buildSkillSourceDetailItems(m.skillsCatalog, item.SourceSafe)
		m.skillDetailSelectedIndex = 0
		m.skillDetailStatus = ""
		m.view = ViewSkillSourceDetail
	}
	return m, nil
}

func (m Model) installSkillSource() (tea.Model, tea.Cmd) {
	source := strings.TrimSpace(m.skillsInput)
	result, err := m.skillsStore.InstallOrUpdateSource(source)
	if err != nil {
		m.skillSelectionStatus = err.Error()
		return m, nil
	}
	if err := m.refreshSkillsCatalog(); err != nil {
		m.skillSelectionStatus = err.Error()
		return m, nil
	}
	m.selectedSkillSourceSafe = result.SourceSafe
	m.skillSourceDetailItems = buildSkillSourceDetailItems(m.skillsCatalog, result.SourceSafe)
	m.skillDetailSelectedIndex = 0
	m.skillDetailStatus = fmt.Sprintf("Installed %d skills from %s", result.InstalledCount, source)
	m.skillsInput = ""
	m.skillsCursor = 0
	m.syncTextInputFromSkillsInput()
	m.view = ViewSkillSourceDetail
	return m, nil
}

func (m Model) saveSkillSelection() (tea.Model, tea.Cmd) {
	selected := make(map[string]bool)
	for _, item := range m.skillSelectionItems {
		if item.Kind == "skill" && item.Selected {
			selected[item.LinkName] = true
		}
	}
	var err error
	if m.skillSelectionScope == skillScopeGlobal {
		err = m.skillsStore.ApplyGlobalSelection(selected)
	} else {
		err = m.skillsStore.ApplyProjectSelection(m.cwd, selected)
	}
	if err != nil {
		m.skillSelectionStatus = err.Error()
		return m, nil
	}
	_ = m.refreshSkillsCatalog()
	if m.skillSelectionScope == skillScopeGlobal {
		m.skillSelectionStatus = "Saved global skills"
	} else {
		m.skillSelectionStatus = "Saved project skills"
	}
	m.skillSelectionItems = buildSkillSelectionItems(m.skillsCatalog, m.skillSelectionScope, m.cwd)
	m.skillsSelectedIndex = firstSelectableSkillIndex(m.skillSelectionItems)
	return m, nil
}

func (m Model) selectSkillSourceDetailItem() (tea.Model, tea.Cmd) {
	idx := render.ClampSelectedIndex(m.skillDetailSelectedIndex, len(m.skillSourceDetailItems))
	if idx >= len(m.skillSourceDetailItems) {
		return m, nil
	}
	item := m.skillSourceDetailItems[idx]
	switch item.Kind {
	case "update":
		source := item.Source
		result, err := m.skillsStore.InstallOrUpdateSource(source)
		if err != nil {
			m.skillDetailStatus = err.Error()
			return m, nil
		}
		_ = m.refreshSkillsCatalog()
		m.skillSourceDetailItems = buildSkillSourceDetailItems(m.skillsCatalog, item.SourceSafe)
		m.skillDetailStatus = fmt.Sprintf("Updated %s (%d skills, %d removed)", source, result.InstalledCount, result.RemovedCount)
	case "remove":
		impact, err := m.skillsStore.RemoveSourceImpact(item.SourceSafe, m.cwd)
		if err != nil {
			m.skillDetailStatus = err.Error()
			return m, nil
		}
		m.skillRemoveImpact = impact
		m.skillRemoveStatus = ""
		m.skillRemoveSelectedIndex = 0
		m.view = ViewSkillSourceRemoveConfirm
	}
	return m, nil
}

func (m Model) selectSkillSourceRemoveConfirmItem() (tea.Model, tea.Cmd) {
	if m.skillRemoveSelectedIndex == 0 {
		result, err := m.skillsStore.RemoveSource(m.selectedSkillSourceSafe)
		if err != nil {
			m.skillRemoveStatus = err.Error()
			return m, nil
		}
		_ = m.refreshSkillsCatalog()
		m.view = ViewSkillsManager
		m.skillsSelectedIndex = 0
		m.skillsManagerStatus = fmt.Sprintf("Removed source %s (%d skills)", result.SourceSafe, result.RemovedCount)
		return m, nil
	}
	m.view = ViewSkillSourceDetail
	return m, nil
}

func (m Model) currentManagerItems() []managerItem {
	if len(m.managerItems) == 0 {
		m.managerItems = []managerItem{{Label: "Skills manager", Kind: "skills"}, {Label: "MCP manager (coming soon)", Kind: "disabled"}}
	}
	return m.managerItems
}

func (m Model) currentSkillsManagerItems() []skillManagerItem {
	if len(m.skillManagerItems) == 0 {
		m.skillManagerItems = buildSkillsManagerItems(m.skillsCatalog)
	}
	return m.skillManagerItems
}

func buildSkillsManagerItems(catalog skills.Catalog) []skillManagerItem {
	items := []skillManagerItem{{Label: "Install source", Kind: "install"}, {Label: "Global skills", Kind: "global"}, {Label: "Project skills", Kind: "project"}}
	for _, source := range catalog.Sources {
		items = append(items, skillManagerItem{Label: source.Source, Kind: "source", SourceSafe: source.SourceSafe, Source: source.Source})
	}
	return items
}

func buildSkillSelectionItems(catalog skills.Catalog, scope skillScope, cwd string) []skillSelectionItem {
	projectPath, _ := filepath.Abs(cwd)
	var items []skillSelectionItem
	for _, source := range catalog.Sources {
		items = append(items, skillSelectionItem{Kind: "header", Label: source.Source, SourceSafe: source.SourceSafe, Source: source.Source})
		for _, skill := range source.Skills {
			selected := skill.GlobalEnabled
			if scope == skillScopeProject {
				selected = containsProjectPath(skill.ProjectPaths, projectPath)
			}
			items = append(items, skillSelectionItem{Kind: "skill", Label: skill.Name, Description: skill.Description, SourceSafe: source.SourceSafe, Source: source.Source, SkillName: skill.Name, LinkName: skills.ActivationLinkName(source.SourceSafe, skill.Name), Selected: selected})
		}
	}
	return items
}

func buildSkillSourceDetailItems(catalog skills.Catalog, sourceSafe string) []skillSourceDetailItem {
	source := catalog.SourceBySafe(sourceSafe)
	if source == nil {
		return nil
	}
	items := []skillSourceDetailItem{{Label: "Update source", Kind: "update", SourceSafe: source.SourceSafe, Source: source.Source}, {Label: "Remove source", Kind: "remove", SourceSafe: source.SourceSafe, Source: source.Source}}
	for _, skill := range source.Skills {
		items = append(items, skillSourceDetailItem{Label: skill.Name, Description: skill.Description, Kind: "skill", SourceSafe: source.SourceSafe, Source: source.Source, SkillName: skill.Name})
	}
	return items
}

func firstSelectableSkillIndex(items []skillSelectionItem) int {
	for i, item := range items {
		if item.Kind == "skill" {
			return i
		}
	}
	return 0
}

func previousSelectableSkillIndex(items []skillSelectionItem, current int) int {
	if len(items) == 0 {
		return 0
	}
	idx := render.ClampSelectedIndex(current, len(items))
	for i := idx - 1; i >= 0; i-- {
		if items[i].Kind == "skill" {
			return i
		}
	}
	return idx
}

func nextSelectableSkillIndex(items []skillSelectionItem, current int) int {
	if len(items) == 0 {
		return 0
	}
	idx := render.ClampSelectedIndex(current, len(items))
	for i := idx + 1; i < len(items); i++ {
		if items[i].Kind == "skill" {
			return i
		}
	}
	return idx
}

func containsProjectPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

// --- Item helpers ---

type pickItem struct {
	Type    string
	Session *provider.Session
}

func (m Model) currentSessionItems() []pickItem {
	filtered := render.FilterSessions(m.sessions, m.sessionQuery)
	items := make([]pickItem, 0, len(filtered)+1)
	items = append(items, pickItem{Type: "new"})
	for i := range filtered {
		items = append(items, pickItem{Type: "session", Session: &filtered[i]})
	}
	return items
}

type workspaceItem struct {
	Type      string
	Workspace provider.Workspace
}

func (m Model) currentWorkspaceItems() []workspaceItem {
	if m.workspaces == nil {
		ctx := provider.Context{DataHome: m.provider.DefaultHome()}
		m.workspaces = m.provider.ListWorkspaces(ctx)
	}
	terms := session.SearchTerms(m.workspaceQuery)
	var items []workspaceItem
	for _, ws := range m.workspaces {
		text := strings.ToLower(strings.Join([]string{
			ws.Cwd, ws.ProjectDir, ws.UpdatedAt, ws.StartedAt,
			ws.FirstUserMessage, ws.LastUserMessage,
		}, " "))
		if session.MatchSearch(text, terms) {
			items = append(items, workspaceItem{Type: "workspace", Workspace: ws})
		}
	}
	return items
}

// --- Render methods ---

func (m Model) renderSessions() string {
	now := time.Now()
	items := m.currentSessionItems()
	idx := render.ClampSelectedIndex(m.sessionSelectedIndex, len(items))
	filteredCount := len(items) - 1

	numberWidth := 2
	timeWidth := 7
	msgsWidth := 8

	for i, item := range items {
		if item.Type == "session" {
			num := fmt.Sprintf("%d.", i)
			if len(num) > numberWidth {
				numberWidth = len(num)
			}
			tw := render.DisplayWidth(render.FormatSessionTime(item.Session.UpdatedAt, now))
			if tw > timeWidth {
				timeWidth = tw
			}
			mw := len(fmt.Sprintf("%d msg", item.Session.MessageCount))
			if mw > msgsWidth {
				msgsWidth = mw
			}
		}
	}

	fixedWidth := 2 + numberWidth + 2 + timeWidth + 2 + msgsWidth + 2
	firstPromptWidth, lastPromptWidth := render.SplitPromptWidths(max(0, m.width-fixedWidth))

	title := m.provider.Name() + " sessions"
	if idx < len(items) && items[idx].Type == "session" {
		title += "  " + items[idx].Session.ID
	}

	header := []string{
		render.FitLine(title, m.width),
		render.FitLine("Workspace: "+m.cwd, m.width),
		render.FitLine(render.PickerStatusLine(m.permissionMode, filteredCount, m.sessionQuery, m.useColor), m.width),
		"",
	}
	rows := make([]listRow, 0, len(items))
	for itemIndex, item := range items {
		prefix := "  "
		if itemIndex == idx {
			prefix = "> "
		}

		if item.Type == "new" {
			rows = append(rows, listRow{
				Text:     fmt.Sprintf("%s%s new", prefix, render.PadDisplay("0.", numberWidth, "right")),
				Selected: itemIndex == idx,
			})
			continue
		}

		s := item.Session
		updated := render.FormatSessionTime(s.UpdatedAt, now)
		messages := fmt.Sprintf("%d msg", s.MessageCount)
		firstPrompt := render.TruncateToWidth(render.DisplayFirstUserMessage(*s), firstPromptWidth)
		lastPrompt := "-"
		if lastPromptWidth > 0 {
			lastPrompt = render.TruncateToWidth(render.DisplayLastUserMessage(*s), lastPromptWidth)
		}

		var promptPart string
		if lastPromptWidth > 0 {
			promptPart = render.PadDisplay(firstPrompt, firstPromptWidth, "left") + "  " + lastPrompt
		} else {
			promptPart = firstPrompt
		}

		line := fmt.Sprintf("%s%s %s  %s  %s",
			prefix,
			render.PadDisplay(fmt.Sprintf("%d.", itemIndex), numberWidth, "right"),
			render.PadDisplay(updated, timeWidth, "left"),
			render.PadDisplay(messages, msgsWidth, "right"),
			promptPart,
		)
		rows = append(rows, listRow{
			Text:     line,
			Selected: itemIndex == idx,
		})
	}

	footer := []string(nil)
	if filteredCount == 0 && strings.TrimSpace(m.sessionQuery) != "" {
		footer = append(footer, "", "No matching sessions.")
	}

	return listView{
		Width:                m.width,
		Height:               m.height - 3,
		Header:               header,
		Rows:                 rows,
		Footer:               footer,
		UseColor:             m.useColor,
		DefaultSelectedColor: render.ANSISelected,
	}.Render()
}

func (m Model) renderPreview() string {
	now := time.Now()
	items := m.currentSessionItems()
	idx := render.ClampSelectedIndex(m.sessionSelectedIndex, len(items))
	if idx >= len(items) || items[idx].Type != "session" {
		m.view = ViewSessions
		return m.renderSessions()
	}

	s := items[idx].Session
	title := m.provider.Name() + " sessions  " + s.ID

	headerLines := []string{
		render.FitLine(title, m.width),
		render.FitLine("Workspace: "+m.cwd, m.width),
		render.FitLine(render.PickerStatusLine(m.permissionMode, len(items)-1, m.sessionQuery, m.useColor), m.width),
		"",
		render.FitLine(fmt.Sprintf("Messages: %d  Started: %s  Updated: %s", s.MessageCount, s.StartedAt, s.UpdatedAt), m.width),
	}

	var bodyLines []string
	lastMessageStart := 0
	if m.previewError != "" {
		bodyLines = append(bodyLines, "")
		bodyLines = append(bodyLines, render.FitLine("Failed to load transcript:", m.width))
		for _, line := range render.WrapTextPreserveNewlines(m.previewError, m.width) {
			bodyLines = append(bodyLines, render.FitLine(line, m.width))
		}
	} else if len(m.previewTranscript) > 0 {
		bodyLines = append(bodyLines, "")
		bodyLines = append(bodyLines, render.FitLine(fmt.Sprintf("Transcript: %d conversation messages", previewMessageCount(m.previewTranscript)), m.width))
		for i, msg := range m.previewTranscript {
			bodyLines = append(bodyLines, "")
			if msg.Role == "omitted" {
				for _, omittedLine := range []string{".", ".", ".", msg.Text, ".", ".", "."} {
					bodyLines = append(bodyLines, render.Colorize(render.FitLine(omittedLine, m.width), render.ANSIPreviewOmitted, m.useColor))
				}
				continue
			}
			lastMessageStart = len(bodyLines) - 1
			ordinal := msg.Ordinal
			if ordinal <= 0 {
				ordinal = i + 1
			}
			role := strings.ToLower(msg.Role)
			color := render.ANSIPreviewMeta
			header := render.Colorize(
				fmt.Sprintf("#%d %s %s", ordinal, role, render.FormatSessionTime(msg.Timestamp, now)),
				color,
				m.useColor,
			)
			bodyLines = append(bodyLines, render.FitLine(header, m.width))
			for _, line := range render.WrapTextPreserveNewlines(truncatePreviewMessageText(msg.Text), m.width) {
				fitted := render.FitLine(line, m.width)
				if role == "assistant" {
					fitted = render.Colorize(fitted, render.ANSIPreviewMuted, m.useColor)
				}
				bodyLines = append(bodyLines, fitted)
			}
		}
	}

	bodyHeight := max(1, m.height-len(headerLines))
	maxScroll := max(0, len(bodyLines)-bodyHeight)
	scroll := min(m.previewScroll, maxScroll)
	if m.previewAutoBottom {
		scroll = min(lastMessageStart, maxScroll)
	}
	header := renderViewport(m.width, len(headerLines), strings.Join(headerLines, "\n"), 0)
	body := renderViewport(m.width, max(1, bodyHeight-1), strings.Join(bodyLines, "\n"), scroll)
	return header + "\n" + body + "\n" + renderHelp(m.width, helpKeyMapForView(helpKindPreview))
}

func previewMessageCount(messages []provider.TranscriptMessage) int {
	count := 0
	for _, msg := range messages {
		if msg.Role != "omitted" {
			count++
		}
	}
	return count
}

const previewMessageRuneLimit = 500
const previewMessageEdgeRunes = 250

func truncatePreviewMessageText(text string) string {
	runes := []rune(text)
	positions := nonWhitespaceRunePositions(runes)
	if len(positions) <= previewMessageRuneLimit {
		return text
	}

	prefixEnd := positions[previewMessageEdgeRunes-1] + 1
	for prefixEnd < len(runes) && unicode.IsSpace(runes[prefixEnd]) {
		prefixEnd++
	}
	suffixStart := positions[len(positions)-previewMessageEdgeRunes]
	for suffixStart > 0 && unicode.IsSpace(runes[suffixStart-1]) {
		suffixStart--
	}
	skipped := len(positions) - previewMessageEdgeRunes*2
	if skipped < 0 {
		skipped = 0
	}
	return string(runes[:prefixEnd]) + "\n\n.\n.\n.\n" + fmt.Sprintf("[%d chars truncated]", skipped) + "\n.\n.\n.\n\n" + string(runes[suffixStart:])
}

func nonWhitespaceRunePositions(runes []rune) []int {
	positions := make([]int, 0, len(runes))
	for i, r := range runes {
		if !unicode.IsSpace(r) {
			positions = append(positions, i)
		}
	}
	return positions
}

func (m Model) renderWorkspaces() string {
	now := time.Now()
	items := m.currentWorkspaceItems()
	idx := render.ClampSelectedIndex(m.workspaceSelectedIndex, len(items))

	title := m.provider.Name() + " workspaces"
	numberWidth := 2
	timeWidth := 7
	sessionsWidth := 8
	msgsWidth := 8

	for i, item := range items {
		num := fmt.Sprintf("%d.", i)
		if len(num) > numberWidth {
			numberWidth = len(num)
		}
		tw := render.DisplayWidth(render.FormatSessionTime(item.Workspace.UpdatedAt, now))
		if tw > timeWidth {
			timeWidth = tw
		}
		sw := len(fmt.Sprintf("%d sessions", item.Workspace.SessionCount))
		if sw > sessionsWidth {
			sessionsWidth = sw
		}
		mw := len(fmt.Sprintf("%d msg", item.Workspace.MessageCount))
		if mw > msgsWidth {
			msgsWidth = mw
		}
	}

	fixedWidth := 2 + numberWidth + 2 + timeWidth + 2 + sessionsWidth + 2 + msgsWidth + 2
	pathWidth := max(1, m.width-fixedWidth)

	header := []string{
		render.FitLine(title, m.width),
		render.FitLine("Search: "+m.workspaceQuery, m.width),
		render.FitLine(fmt.Sprintf("Matches: %d", len(items)), m.width),
		"",
	}
	rows := make([]listRow, 0, len(items))
	for itemIndex, item := range items {
		prefix := "  "
		if itemIndex == idx {
			prefix = "> "
		}

		ws := item.Workspace
		updated := render.FormatSessionTime(ws.UpdatedAt, now)
		sessions := fmt.Sprintf("%d sessions", ws.SessionCount)
		messages := fmt.Sprintf("%d msg", ws.MessageCount)
		wsPath := render.TruncateToWidth(ws.Cwd, pathWidth)
		if wsPath == "" {
			wsPath = render.TruncateToWidth(ws.ProjectDir, pathWidth)
		}
		if wsPath == "" {
			wsPath = "-"
		}

		line := fmt.Sprintf("%s%s %s  %s  %s  %s",
			prefix,
			render.PadDisplay(fmt.Sprintf("%d.", itemIndex), numberWidth, "right"),
			render.PadDisplay(updated, timeWidth, "left"),
			render.PadDisplay(sessions, sessionsWidth, "right"),
			render.PadDisplay(messages, msgsWidth, "right"),
			wsPath,
		)
		rows = append(rows, listRow{
			Text:     line,
			Selected: itemIndex == idx,
		})
	}

	return listView{
		Width:                m.width,
		Height:               m.height - 1,
		Header:               header,
		Rows:                 rows,
		UseColor:             m.useColor,
		DefaultSelectedColor: render.ANSISelected,
	}.Render()
}

func (m Model) renderConfigurations() string {
	title := m.provider.ConfigurationTitle()
	items := m.config.actions

	header := []string{
		render.FitLine(title, m.width),
	}

	if m.config.status != "" {
		header = append(header, render.FitLine(m.config.status, m.width))
	}
	header = append(header, "")

	if len(items) == 0 {
		return listView{
			Width:        m.width,
			Height:       m.height,
			Header:       header,
			EmptyMessage: "No configurations.",
		}.Render()
	}

	idx := render.ClampSelectedIndex(m.config.selectedIndex, len(items))
	numberWidth := 2
	nameWidth := 0

	ctx := provider.Context{Cwd: m.cwd, DataHome: m.provider.DefaultHome()}

	// Pre-compute all action columns to determine alignment widths.
	type actionRow struct {
		action   provider.ConfigAction
		colTexts []string
	}
	rows := make([]actionRow, len(items))

	// Determine max column count and compute per-column widths.
	maxCols := 0
	for i, action := range items {
		num := fmt.Sprintf("%d.", i)
		if len(num) > numberWidth {
			numberWidth = len(num)
		}
		nw := render.DisplayWidth(action.Name)
		if nw > nameWidth {
			nameWidth = nw
		}
		if action.Columns != nil {
			cols := action.Columns(ctx)
			var texts []string
			for _, col := range cols {
				if col.Value != "" {
					texts = append(texts, col.Value)
				}
			}
			rows[i] = actionRow{action: action, colTexts: texts}
			if len(texts) > maxCols {
				maxCols = len(texts)
			}
		} else {
			rows[i] = actionRow{action: action}
		}
	}

	colWidths := make([]int, maxCols)
	for _, row := range rows {
		for ci, t := range row.colTexts {
			w := render.DisplayWidth(t)
			if w > colWidths[ci] {
				colWidths[ci] = w
			}
		}
	}

	listRows := make([]listRow, 0, len(rows))
	for itemIndex, row := range rows {
		prefix := "  "
		if itemIndex == idx {
			prefix = "> "
		}

		namePart := render.PadDisplay(row.action.Name, nameWidth, "left")

		line := fmt.Sprintf("%s%s %s", prefix, render.PadDisplay(fmt.Sprintf("%d.", itemIndex), numberWidth, "right"), namePart)

		// Append columns with alignment.
		if len(row.colTexts) > 0 {
			var aligned []string
			for ci, t := range row.colTexts {
				if ci < len(colWidths) {
					aligned = append(aligned, render.PadDisplay(t, colWidths[ci], "left"))
				}
			}
			line += "  " + strings.Join(aligned, "  ")
		}

		listRows = append(listRows, listRow{
			Text:     line,
			Selected: itemIndex == idx,
		})
	}

	return listView{
		Width:                m.width,
		Height:               m.height,
		Header:               header,
		Rows:                 listRows,
		UseColor:             m.useColor,
		DefaultSelectedColor: render.ANSISelected,
	}.Render()
}

func (m Model) renderConfigurationItems() string {
	title := "Configurations"
	if m.config.activeAction != nil && m.config.activeAction.Title != "" {
		title = m.config.activeAction.Title
	}

	header := []string{
		render.FitLine(title, m.width),
	}

	if m.config.status != "" {
		header = append(header, render.FitLine(m.config.status, m.width))
	}
	header = append(header, "")

	if len(m.config.items) == 0 {
		emptyMsg := "No configurations."
		if m.config.activeAction != nil && m.config.activeAction.Select != nil && m.config.activeAction.Select.EmptyMessage != "" {
			emptyMsg = m.config.activeAction.Select.EmptyMessage
		}
		return listView{
			Width:        m.width,
			Height:       m.height,
			Header:       header,
			EmptyMessage: emptyMsg,
		}.Render()
	}

	idx := render.ClampSelectedIndex(m.config.itemSelectedIndex, len(m.config.items))
	numberWidth := 2
	labelWidth := 0

	// Calculate column widths from visible items.
	columnCount := 0
	for _, item := range m.config.items {
		if len(item.Columns) > columnCount {
			columnCount = len(item.Columns)
		}
	}
	columnWidths := make([]int, columnCount)

	for i, item := range m.config.items {
		num := fmt.Sprintf("%d.", i)
		if len(num) > numberWidth {
			numberWidth = len(num)
		}
		lw := render.DisplayWidth(item.Label)
		if lw > labelWidth {
			labelWidth = lw
		}
		for ci, col := range item.Columns {
			cw := render.DisplayWidth(col.Value)
			if cw > columnWidths[ci] {
				columnWidths[ci] = cw
			}
		}
	}

	rows := make([]listRow, 0, len(m.config.items))
	for itemIndex, item := range m.config.items {
		prefix := "  "
		if itemIndex == idx {
			prefix = "> "
		}

		// Build the label part.
		labelPart := render.PadDisplay(item.Label, labelWidth, "left")

		// Build the columns suffix.
		var colParts []string
		for ci, col := range item.Columns {
			if ci < len(columnWidths) {
				colParts = append(colParts, render.PadDisplay(col.Value, columnWidths[ci], "left"))
			}
		}
		suffix := strings.Join(colParts, "  ")

		line := fmt.Sprintf("%s%s %s", prefix, render.PadDisplay(fmt.Sprintf("%d.", itemIndex), numberWidth, "right"), labelPart)
		if suffix != "" {
			line += "  " + suffix
		}
		rows = append(rows, listRow{
			Text:     line,
			Selected: itemIndex == idx,
		})
	}

	return listView{
		Width:                m.width,
		Height:               m.height,
		Header:               header,
		Rows:                 rows,
		UseColor:             m.useColor,
		DefaultSelectedColor: render.ANSISelected,
	}.Render()
}

func (m Model) renderConfigurationSubitems() string {
	title := "Configurations"
	if m.config.activeSubitems != nil && m.config.activeSubitems.Title != nil && m.config.activeItem != nil {
		title = m.config.activeSubitems.Title(*m.config.activeItem)
	}

	header := []string{
		render.FitLine(title, m.width),
	}

	if m.config.status != "" {
		header = append(header, render.FitLine(m.config.status, m.width))
	}
	header = append(header, "")

	if len(m.config.subitems) == 0 {
		emptyMsg := "No configurations."
		if m.config.activeSubitems != nil && m.config.activeSubitems.EmptyMessage != "" {
			emptyMsg = m.config.activeSubitems.EmptyMessage
		}
		return listView{
			Width:        m.width,
			Height:       m.height,
			Header:       header,
			EmptyMessage: emptyMsg,
		}.Render()
	}

	idx := render.ClampSelectedIndex(m.config.itemSelectedIndex, len(m.config.subitems))
	numberWidth := 2

	for i := range m.config.subitems {
		num := fmt.Sprintf("%d.", i)
		if len(num) > numberWidth {
			numberWidth = len(num)
		}
	}

	rows := make([]listRow, 0, len(m.config.subitems))
	for itemIndex, item := range m.config.subitems {
		prefix := "  "
		if itemIndex == idx {
			prefix = "> "
		}

		marker := "[ ] "
		if item.Selected {
			marker = "[✔] "
		}

		line := fmt.Sprintf("%s%s %s%s", prefix, render.PadDisplay(fmt.Sprintf("%d.", itemIndex), numberWidth, "right"), marker, item.Label)
		row := listRow{
			Text:     line,
			Selected: item.Selected || itemIndex == idx,
		}
		if item.Selected {
			row.Color = render.ANSISelectedConfig
		} else if itemIndex == idx {
			row.Color = render.ANSISelected
		}
		rows = append(rows, row)
	}

	return listView{
		Width:                m.width,
		Height:               m.height,
		Header:               header,
		Rows:                 rows,
		UseColor:             m.useColor,
		DefaultSelectedColor: render.ANSISelected,
	}.Render()
}

func (m Model) renderManagerHub() string {
	items := m.currentManagerItems()
	idx := render.ClampSelectedIndex(m.managerSelectedIndex, len(items))
	header := []string{"Manager", "", "Use Left from sessions to enter global management.", ""}
	rows := make([]listRow, 0, len(items))
	for i, item := range items {
		prefix := "  "
		if i == idx {
			prefix = "> "
		}
		rows = append(rows, listRow{
			Text:     fmt.Sprintf("%s%d. %s", prefix, i, item.Label),
			Selected: i == idx,
		})
	}
	return menuListView{
		Width:    m.width,
		Height:   m.height,
		Header:   header,
		Rows:     rows,
		Selected: idx,
		UseColor: m.useColor,
	}.Render()
}

func (m Model) renderSkillsManager() string {
	items := m.currentSkillsManagerItems()
	idx := render.ClampSelectedIndex(m.skillsSelectedIndex, len(items))
	header := []string{"Skills manager", render.FitLine("Project: "+m.cwd, m.width)}
	if m.skillsManagerStatus != "" {
		header = append(header, render.FitLine(m.skillsManagerStatus, m.width))
	}
	header = append(header, "")
	rows := make([]listRow, 0, len(items))
	for i, item := range items {
		prefix := "  "
		if i == idx {
			prefix = "> "
		}
		label := item.Label
		if item.Kind == "source" {
			source := m.skillsCatalog.SourceBySafe(item.SourceSafe)
			if source != nil {
				label = fmt.Sprintf("%s  (%d skills)", item.Label, len(source.Skills))
			}
		}
		rows = append(rows, listRow{
			Text:     fmt.Sprintf("%s%d. %s", prefix, i, label),
			Selected: i == idx,
		})
	}
	return menuListView{
		Width:    m.width,
		Height:   m.height,
		Header:   header,
		Rows:     rows,
		Selected: idx,
		UseColor: m.useColor,
	}.Render()
}

func (m Model) renderSkillsInstallInput() string {
	lines := []string{"Install source", "", "Enter GitHub source in owner/repo form.", render.FitLine("Source: "+m.skillsTextInput.View(), m.width)}
	if m.skillSelectionStatus != "" {
		lines = append(lines, "", render.FitLine(m.skillSelectionStatus, m.width))
	}
	lines = append(lines, renderHelp(m.width, helpKeyMapForView(helpKindInput)))
	return renderViewport(m.width, m.height, strings.Join(lines, "\n"), 0)
}

func (m Model) renderSkillSelectionView() string {
	title := "Global skills"
	if m.view == ViewSkillsProject {
		title = "Project skills"
	}
	idx := render.ClampSelectedIndex(m.skillsSelectedIndex, len(m.skillSelectionItems))
	skillNameWidth, skillDescriptionWidth := skillListColumnWidths(m.width, render.DisplayWidth("  [ ]"), skillSelectionNames(m.skillSelectionItems))
	header := []string{title, render.FitLine("Project: "+m.cwd, m.width)}
	if m.skillSelectionStatus != "" {
		header = append(header, render.FitLine(m.skillSelectionStatus, m.width))
	}
	header = append(header, "", "Space toggles, Enter saves.", "")
	rows := make([]listRow, 0, len(m.skillSelectionItems))
	for i, item := range m.skillSelectionItems {
		if item.Kind == "header" {
			rows = append(rows, listRow{Text: item.Label})
			continue
		}
		prefix := "  "
		if i == idx {
			prefix = "> "
		}
		marker := "[ ]"
		if item.Selected {
			marker = "[✔]"
		}
		row := listRow{
			Text:     renderSkillListLine(prefix+marker, item.Label, item.Description, skillNameWidth, skillDescriptionWidth),
			Selected: item.Selected || i == idx,
			Focused:  i == idx,
		}
		if item.Selected {
			row.Color = render.ANSISelectedConfig
		} else if i == idx {
			row.Color = render.ANSISelected
		}
		rows = append(rows, row)
	}
	return listView{
		Width:                m.width,
		Height:               m.height,
		Header:               header,
		Rows:                 rows,
		UseColor:             m.useColor,
		DefaultSelectedColor: render.ANSISelected,
		ViewportMode:         listViewportCenterSelection,
	}.Render()
}

func skillSelectionWindowStart(itemCount int, selectedIndex int, maxRows int) int {
	return listWindowStart(itemCount, selectedIndex, maxRows, listViewportCenterSelection)
}

func skillSelectionNames(items []skillSelectionItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind == "skill" {
			names = append(names, item.Label)
		}
	}
	return names
}

func skillSourceDetailNames(items []skillSourceDetailItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind == "skill" {
			names = append(names, item.Label)
		}
	}
	return names
}

func skillListColumnWidths(totalWidth int, prefixWidth int, names []string) (int, int) {
	contentWidth := max(1, totalWidth-prefixWidth-1)
	if len(names) == 0 {
		return contentWidth, 0
	}
	const gapWidth = 2
	maxNameWidth := 0
	for _, name := range names {
		nameWidth := render.DisplayWidth(strings.TrimSpace(name))
		if nameWidth > maxNameWidth {
			maxNameWidth = nameWidth
		}
	}
	nameWidth := min(maxNameWidth, max(12, contentWidth*3/10))
	if nameWidth < 1 {
		nameWidth = 1
	}
	if contentWidth-nameWidth < gapWidth+1 {
		nameWidth = max(1, contentWidth-gapWidth-1)
	}
	descriptionWidth := max(0, contentWidth-nameWidth-gapWidth)
	return nameWidth, descriptionWidth
}

func renderSkillListLine(prefix string, name string, description string, nameWidth int, descriptionWidth int) string {
	namePart := render.PadDisplay(render.TruncateToWidth(name, nameWidth), nameWidth, "left")
	if descriptionWidth <= 0 {
		return prefix + " " + strings.TrimRight(namePart, " ")
	}
	descriptionPart := render.TruncateToWidth(description, descriptionWidth)
	if descriptionPart == "" {
		return prefix + " " + strings.TrimRight(namePart, " ")
	}
	return prefix + " " + namePart + "  " + descriptionPart
}

func (m Model) renderSkillSourceDetail() string {
	idx := render.ClampSelectedIndex(m.skillDetailSelectedIndex, len(m.skillSourceDetailItems))
	title := "Skill source"
	if source := m.skillsCatalog.SourceBySafe(m.selectedSkillSourceSafe); source != nil {
		title = source.Source
	}
	numberWidth := 2
	for i := range m.skillSourceDetailItems {
		if w := len(fmt.Sprintf("%d.", i)); w > numberWidth {
			numberWidth = w
		}
	}
	skillNameWidth, skillDescriptionWidth := skillListColumnWidths(m.width, render.DisplayWidth("  ")+numberWidth, skillSourceDetailNames(m.skillSourceDetailItems))
	header := []string{render.FitLine(title, m.width)}
	if m.skillDetailStatus != "" {
		header = append(header, render.FitLine(m.skillDetailStatus, m.width))
	}
	header = append(header, "")
	rows := make([]listRow, 0, len(m.skillSourceDetailItems))
	for i, item := range m.skillSourceDetailItems {
		prefix := "  "
		if i == idx {
			prefix = "> "
		}
		number := render.PadDisplay(fmt.Sprintf("%d.", i), numberWidth, "right")
		line := fmt.Sprintf("%s%s %s", prefix, number, item.Label)
		if item.Kind == "skill" {
			line = renderSkillListLine(prefix+number, item.Label, item.Description, skillNameWidth, skillDescriptionWidth)
		}
		rows = append(rows, listRow{
			Text:     line,
			Selected: i == idx,
		})
	}
	return listView{
		Width:                m.width,
		Height:               m.height,
		Header:               header,
		Rows:                 rows,
		UseColor:             m.useColor,
		DefaultSelectedColor: render.ANSISelected,
		ViewportMode:         listViewportCenterSelection,
	}.Render()
}

func (m Model) renderSkillSourceRemoveConfirm() string {
	idx := render.ClampSelectedIndex(m.skillRemoveSelectedIndex, 2)
	header := []string{"Remove source", render.FitLine(fmt.Sprintf("Installed skills: %d", m.skillRemoveImpact.InstalledSkills), m.width), render.FitLine(fmt.Sprintf("Global links to remove: %d", m.skillRemoveImpact.GlobalEnabledSkills), m.width), render.FitLine(fmt.Sprintf("Current project links to remove: %d", m.skillRemoveImpact.CurrentProjectSkills), m.width), render.FitLine(fmt.Sprintf("Other project paths to clean: %d", m.skillRemoveImpact.OtherProjectPathCount), m.width)}
	if m.skillRemoveStatus != "" {
		header = append(header, render.FitLine(m.skillRemoveStatus, m.width))
	}
	header = append(header, "")
	options := []string{"Confirm remove", "Cancel"}
	rows := make([]listRow, 0, len(options))
	for i, option := range options {
		prefix := "  "
		if i == idx {
			prefix = "> "
		}
		rows = append(rows, listRow{
			Text:     fmt.Sprintf("%s%d. %s", prefix, i, option),
			Selected: i == idx,
		})
	}
	return menuListView{
		Width:    m.width,
		Height:   m.height,
		Header:   header,
		Rows:     rows,
		Selected: idx,
		UseColor: m.useColor,
	}.Render()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
