package picker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lizhian/agent-session/internal/provider"
	"github.com/lizhian/agent-session/internal/session"
	"github.com/lizhian/agent-session/internal/skills"
)

type fakeProvider struct {
	actions []provider.ConfigAction
}

func (p fakeProvider) Name() string                                             { return "Fake" }
func (p fakeProvider) ConfigPath() string                                       { return "" }
func (p fakeProvider) DefaultHome() string                                      { return "" }
func (p fakeProvider) HomeOptionName() string                                   { return "" }
func (p fakeProvider) PermissionModes() []string                                { return session.DefaultPermissionModes }
func (p fakeProvider) ListSessions(ctx provider.Context) []provider.Session     { return nil }
func (p fakeProvider) ListWorkspaces(ctx provider.Context) []provider.Workspace { return nil }
func (p fakeProvider) LoadSessionTranscript(s provider.Session, ctx provider.Context) []provider.TranscriptMessage {
	return nil
}
func (p fakeProvider) SelectedItemToCommand(item provider.PickItem, permissionMode string, cwd string) provider.CommandSpec {
	return provider.CommandSpec{}
}
func (p fakeProvider) BuildCommand(sessions []provider.Session, choice string, permissionMode string, cwd string) provider.CommandSpec {
	return provider.CommandSpec{}
}
func (p fakeProvider) LoadPermissionMode(ctx provider.Context) string { return "" }
func (p fakeProvider) SavePermissionMode(mode string, ctx provider.Context) error {
	return nil
}
func (p fakeProvider) TrustCurrentFolder(cwd string, ctx provider.Context) error {
	return nil
}
func (p fakeProvider) ConfigurationTitle() string { return "Fake configurations" }
func (p fakeProvider) ConfigurationActions() []provider.ConfigAction {
	return p.actions
}
func (p fakeProvider) WorkspaceCwd(workspace provider.Workspace, currentCwd string) string {
	return currentCwd
}

func TestSelectConfigurationActionAppliesItem(t *testing.T) {
	var applied string
	action := provider.ConfigAction{
		Name:  "Model provider",
		Title: "Model providers",
		Select: &provider.SelectConfigAction{
			EmptyMessage: "No providers.",
			LoadItems: func(ctx provider.Context) ([]provider.ConfigItem, error) {
				return []provider.ConfigItem{
					{Name: "first", Label: "first"},
					{Name: "second", Label: "second", Selected: true},
				}, nil
			},
			ApplyItem: func(item provider.ConfigItem, ctx provider.Context) (string, error) {
				applied = item.Name
				return "applied " + item.Name, nil
			},
		},
	}
	model := NewModel(fakeProvider{actions: []provider.ConfigAction{action}}, nil, "/tmp/work", "default", 80, 24, false)

	updated, _ := model.selectConfiguration()
	model = updated.(Model)
	if model.view != ViewConfigurationItems {
		t.Fatalf("view = %v, want ViewConfigurationItems", model.view)
	}
	if model.config.itemSelectedIndex != 1 {
		t.Fatalf("selected index = %d, want selected item index 1", model.config.itemSelectedIndex)
	}

	updated, _ = model.selectConfigurationItem()
	model = updated.(Model)
	if applied != "second" {
		t.Fatalf("applied item = %q, want second", applied)
	}
	if model.view != ViewConfigurations {
		t.Fatalf("view = %v, want ViewConfigurations", model.view)
	}
	if model.config.status != "applied second" {
		t.Fatalf("status = %q, want applied second", model.config.status)
	}
	if model.config.activeAction != nil || model.config.activeItem != nil || model.config.activeSubitems != nil {
		t.Fatalf("configuration workflow state was not cleared")
	}
}

func TestDirectMultiSelectConfigurationActionAppliesSelectedSubitems(t *testing.T) {
	var appliedItem string
	var appliedNames []string
	action := provider.ConfigAction{
		Name: "Provider openai",
		DirectMultiSelect: &provider.DirectMultiSelectConfigAction{
			Item: provider.ConfigItem{Name: "openai", Label: "openai"},
			Subitems: provider.SubitemConfigAction{
				EmptyMessage: "No models.",
				Title: func(item provider.ConfigItem) string {
					return "Models: " + item.Name
				},
				LoadItems: func(item provider.ConfigItem, ctx provider.Context) ([]provider.ConfigItem, error) {
					return []provider.ConfigItem{
						{Name: "gpt-5", Label: "gpt-5", Selected: true},
						{Name: "gpt-5-mini", Label: "gpt-5-mini"},
					}, nil
				},
				Apply: func(item provider.ConfigItem, selected []provider.ConfigItem, ctx provider.Context) (string, error) {
					appliedItem = item.Name
					for _, item := range selected {
						appliedNames = append(appliedNames, item.Name)
					}
					return "updated models", nil
				},
			},
		},
	}
	model := NewModel(fakeProvider{actions: []provider.ConfigAction{action}}, nil, "/tmp/work", "default", 80, 24, false)

	updated, _ := model.selectConfiguration()
	model = updated.(Model)
	if model.view != ViewConfigurationSubitems {
		t.Fatalf("view = %v, want ViewConfigurationSubitems", model.view)
	}
	if model.config.activeSubitems == nil {
		t.Fatalf("active subitems workflow was not set")
	}

	model.config.itemSelectedIndex = 1
	updated, _ = model.handleSpace()
	model = updated.(Model)
	updated, _ = model.selectConfigurationSubitems()
	model = updated.(Model)

	if appliedItem != "openai" {
		t.Fatalf("applied item = %q, want openai", appliedItem)
	}
	if len(appliedNames) != 2 || appliedNames[0] != "gpt-5" || appliedNames[1] != "gpt-5-mini" {
		t.Fatalf("applied names = %v, want both models", appliedNames)
	}
	if model.view != ViewConfigurations {
		t.Fatalf("view = %v, want ViewConfigurations", model.view)
	}
	if model.config.status != "updated models" {
		t.Fatalf("status = %q, want updated models", model.config.status)
	}
	if model.config.activeAction != nil || model.config.activeItem != nil || model.config.activeSubitems != nil {
		t.Fatalf("configuration workflow state was not cleared")
	}
}

func TestConfigurationWorkflowCancelSubitemsReturnsToActionForDirectMultiSelect(t *testing.T) {
	action := provider.ConfigAction{
		Name: "Provider openai",
		DirectMultiSelect: &provider.DirectMultiSelectConfigAction{
			Item: provider.ConfigItem{Name: "openai", Label: "openai"},
			Subitems: provider.SubitemConfigAction{
				LoadItems: func(item provider.ConfigItem, ctx provider.Context) ([]provider.ConfigItem, error) {
					return []provider.ConfigItem{{Name: "gpt-5", Label: "gpt-5"}}, nil
				},
			},
		},
	}
	workflow := newConfigurationWorkflow(fakeProvider{actions: []provider.ConfigAction{action}})

	view := workflow.selectAction("/tmp/work")
	if view != ViewConfigurationSubitems {
		t.Fatalf("view = %v, want ViewConfigurationSubitems", view)
	}

	view = workflow.cancelSubitems()
	if view != ViewConfigurations {
		t.Fatalf("view = %v, want ViewConfigurations", view)
	}
	if workflow.activeAction != nil || workflow.activeItem != nil || workflow.activeSubitems != nil || workflow.subitems != nil {
		t.Fatalf("configuration workflow state was not cleared")
	}
}

type pickerFakeInstaller struct {
	prepared []skills.PreparedSkill
}

func (f pickerFakeInstaller) Install(source string, tempDir string) ([]skills.PreparedSkill, error) {
	return f.prepared, nil
}

func TestLeftFromSessionsOpensManagerHub(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	updated, _ := model.handleLeft()
	model = updated.(Model)
	if model.view != ViewManagerHub {
		t.Fatalf("view = %v, want ViewManagerHub", model.view)
	}
	if len(model.currentManagerItems()) != 2 {
		t.Fatalf("expected 2 manager items, got %d", len(model.currentManagerItems()))
	}
}

func TestSkillsManagerBuildsSourceItemsAndGroupedSelections(t *testing.T) {
	rootDir := t.TempDir()
	projectDir := filepath.Join(rootDir, "project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := &skills.Store{
		RootDir:         rootDir,
		RepositoryDir:   filepath.Join(rootDir, "repo"),
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: filepath.Join(rootDir, "global"),
		Now:             func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}
	catalog := skills.Catalog{Sources: []skills.SourceRecord{{Source: "mattpocock/skills", SourceSafe: "mattpocock_skills", Skills: []skills.SkillRecord{{Name: "triage", GlobalEnabled: true}, {Name: "zoom-out"}}}, {Source: "tw93/waza", SourceSafe: "tw93_waza", Skills: []skills.SkillRecord{{Name: "review"}}}}}
	if err := store.SaveCatalog(catalog); err != nil {
		t.Fatal(err)
	}
	model := NewModel(fakeProvider{}, nil, projectDir, "default", 80, 24, false)
	model.skillsStore = store
	if err := model.refreshSkillsCatalog(); err != nil {
		t.Fatal(err)
	}
	items := model.currentSkillsManagerItems()
	if len(items) != 5 {
		t.Fatalf("items len = %d, want 5", len(items))
	}
	selections := buildSkillSelectionItems(model.skillsCatalog, skillScopeGlobal, projectDir)
	if len(selections) != 5 {
		t.Fatalf("selection len = %d, want 5", len(selections))
	}
	if selections[0].Kind != "header" || selections[1].Kind != "skill" || selections[1].Selected != true {
		t.Fatalf("unexpected grouped selection items: %+v", selections[:2])
	}
	if selections[1].Description != "" {
		t.Fatalf("expected empty description in fixture, got %q", selections[1].Description)
	}
	if firstSelectableSkillIndex(selections) != 1 {
		t.Fatalf("firstSelectableSkillIndex = %d, want 1", firstSelectableSkillIndex(selections))
	}
}

func TestInstallSourceOpensSourceDetail(t *testing.T) {
	rootDir := t.TempDir()
	projectDir := filepath.Join(rootDir, "project")
	preparedDir := filepath.Join(rootDir, "prepared")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(preparedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	triageDir := filepath.Join(preparedDir, "triage")
	if err := os.MkdirAll(triageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(triageDir, "SKILL.md"), []byte("# triage"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &skills.Store{
		RootDir:         rootDir,
		RepositoryDir:   filepath.Join(rootDir, "repo"),
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: filepath.Join(rootDir, "global"),
		Installer:       pickerFakeInstaller{prepared: []skills.PreparedSkill{{Name: "triage", Path: triageDir}}},
		Now:             func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}
	model := NewModel(fakeProvider{}, nil, projectDir, "default", 80, 24, false)
	model.skillsStore = store
	model.view = ViewSkillsInstallInput
	model.skillsInput = "mattpocock/skills"
	updated, _ := model.installSkillSource()
	model = updated.(Model)
	if model.view != ViewSkillSourceDetail {
		t.Fatalf("view = %v, want ViewSkillSourceDetail", model.view)
	}
	if model.selectedSkillSourceSafe != "mattpocock_skills" {
		t.Fatalf("selectedSkillSourceSafe = %q, want mattpocock_skills", model.selectedSkillSourceSafe)
	}
	if len(model.skillSourceDetailItems) == 0 || model.skillSourceDetailItems[0].Kind != "update" {
		t.Fatalf("unexpected skillSourceDetailItems: %+v", model.skillSourceDetailItems)
	}
	if model.skillDetailStatus == "" {
		t.Fatal("expected install status message")
	}
}

func TestSkillsInstallInputAcceptsPastedSource(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsInstallInput

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("mattpocock/skills")})
	model = updated.(Model)

	if model.skillsInput != "mattpocock/skills" {
		t.Fatalf("skillsInput = %q, want pasted source", model.skillsInput)
	}
}

func TestSkillsInstallInputAcceptsImeCommittedText(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsInstallInput

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tw93/waza")})
	model = updated.(Model)

	if model.skillsInput != "tw93/waza" {
		t.Fatalf("skillsInput = %q, want committed text", model.skillsInput)
	}
}

func TestSkillsInstallInputDoesNotRenderFakeCursor(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsInstallInput
	model.skillsInput = "mattpocock/skills"
	model.skillsCursor = len([]rune(model.skillsInput))

	view := model.renderSkillsInstallInput()
	if strings.Contains(view, "Source: mattpocock/skills|") {
		t.Fatalf("did not expect fake cursor in input view, got %q", view)
	}
}

func TestSkillsInstallInputMovesCursorLeftAndRight(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsInstallInput
	model.skillsInput = "abcd"
	model.skillsCursor = 4

	updated, _ := model.handleLeft()
	model = updated.(Model)
	if model.skillsCursor != 3 {
		t.Fatalf("skillsCursor after left = %d, want 3", model.skillsCursor)
	}

	updated, _ = model.handleRight()
	model = updated.(Model)
	if model.skillsCursor != 4 {
		t.Fatalf("skillsCursor after right = %d, want 4", model.skillsCursor)
	}
}

func TestSkillsInstallInputBackspaceDeletesBeforeCursor(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsInstallInput
	model.skillsInput = "abxd"
	model.skillsCursor = 3

	updated, _ := model.handleBackspace()
	model = updated.(Model)
	if model.skillsInput != "abd" {
		t.Fatalf("skillsInput after backspace = %q, want abd", model.skillsInput)
	}
	if model.skillsCursor != 2 {
		t.Fatalf("skillsCursor after backspace = %d, want 2", model.skillsCursor)
	}
}

func TestSkillsInstallInputInsertsTextAtCursor(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsInstallInput
	model.skillsInput = "abde"
	model.skillsCursor = 2

	updated, _ := model.handleText("c")
	model = updated.(Model)
	if model.skillsInput != "abcde" {
		t.Fatalf("skillsInput after insert = %q, want abcde", model.skillsInput)
	}
	if model.skillsCursor != 3 {
		t.Fatalf("skillsCursor after insert = %d, want 3", model.skillsCursor)
	}
}

func TestGlobalSkillsUpMovesToPreviousSelectableSkill(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsGlobal
	model.skillSelectionItems = []skillSelectionItem{
		{Kind: "header", Label: "matt/skills"},
		{Kind: "skill", Label: "triage", LinkName: "matt_skills__triage"},
		{Kind: "skill", Label: "zoom-out", LinkName: "matt_skills__zoom-out"},
	}
	model.skillsSelectedIndex = 2

	updated, _ := model.handleUp()
	model = updated.(Model)

	if model.skillsSelectedIndex != 1 {
		t.Fatalf("skillsSelectedIndex after up = %d, want 1", model.skillsSelectedIndex)
	}
}

func TestGlobalSkillsUpSkipsHeaderRows(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 24, false)
	model.view = ViewSkillsGlobal
	model.skillSelectionItems = []skillSelectionItem{
		{Kind: "header", Label: "matt/skills"},
		{Kind: "skill", Label: "triage", LinkName: "matt_skills__triage"},
		{Kind: "header", Label: "tw93/waza"},
		{Kind: "skill", Label: "review", LinkName: "tw93_waza__review"},
	}
	model.skillsSelectedIndex = 3

	updated, _ := model.handleUp()
	model = updated.(Model)

	if model.skillsSelectedIndex != 1 {
		t.Fatalf("skillsSelectedIndex after up = %d, want 1", model.skillsSelectedIndex)
	}
}

func TestRenderGlobalSkillsShowsTopItemsWhenSelectionNearTop(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 12, false)
	model.view = ViewSkillsGlobal
	model.skillSelectionItems = []skillSelectionItem{
		{Kind: "header", Label: "matt/skills"},
		{Kind: "skill", Label: "skill-01", LinkName: "matt_skills__skill-01"},
		{Kind: "skill", Label: "skill-02", LinkName: "matt_skills__skill-02"},
		{Kind: "skill", Label: "skill-03", LinkName: "matt_skills__skill-03"},
		{Kind: "skill", Label: "skill-04", LinkName: "matt_skills__skill-04"},
		{Kind: "skill", Label: "skill-05", LinkName: "matt_skills__skill-05"},
		{Kind: "skill", Label: "skill-06", LinkName: "matt_skills__skill-06"},
		{Kind: "skill", Label: "skill-07", LinkName: "matt_skills__skill-07"},
		{Kind: "skill", Label: "skill-08", LinkName: "matt_skills__skill-08"},
		{Kind: "skill", Label: "skill-09", LinkName: "matt_skills__skill-09"},
		{Kind: "skill", Label: "skill-10", LinkName: "matt_skills__skill-10"},
	}
	model.skillsSelectedIndex = 1

	view := model.renderSkillSelectionView()
	if !strings.Contains(view, "skill-01") {
		t.Fatalf("expected top selected skill to be visible, got %q", view)
	}
	if strings.Contains(view, "skill-10") {
		t.Fatalf("did not expect bottom skill in top viewport, got %q", view)
	}
}

func TestRenderGlobalSkillsScrollsToBottomSelection(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 12, false)
	model.view = ViewSkillsGlobal
	model.skillSelectionItems = []skillSelectionItem{
		{Kind: "header", Label: "matt/skills"},
		{Kind: "skill", Label: "skill-01", LinkName: "matt_skills__skill-01"},
		{Kind: "skill", Label: "skill-02", LinkName: "matt_skills__skill-02"},
		{Kind: "skill", Label: "skill-03", LinkName: "matt_skills__skill-03"},
		{Kind: "skill", Label: "skill-04", LinkName: "matt_skills__skill-04"},
		{Kind: "skill", Label: "skill-05", LinkName: "matt_skills__skill-05"},
		{Kind: "skill", Label: "skill-06", LinkName: "matt_skills__skill-06"},
		{Kind: "skill", Label: "skill-07", LinkName: "matt_skills__skill-07"},
		{Kind: "skill", Label: "skill-08", LinkName: "matt_skills__skill-08"},
		{Kind: "skill", Label: "skill-09", LinkName: "matt_skills__skill-09"},
		{Kind: "skill", Label: "skill-10", LinkName: "matt_skills__skill-10"},
	}
	model.skillsSelectedIndex = 10

	view := model.renderSkillSelectionView()
	if !strings.Contains(view, "skill-10") {
		t.Fatalf("expected bottom selected skill to be visible, got %q", view)
	}
	if strings.Contains(view, "skill-01") {
		t.Fatalf("did not expect top skill in bottom viewport, got %q", view)
	}
}

func TestRenderSkillSelectionViewShowsColumnsWithoutDescriptionFooter(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 12, false)
	model.view = ViewSkillsGlobal
	model.skillSelectionItems = []skillSelectionItem{
		{Kind: "header", Label: "matt/skills"},
		{Kind: "skill", Label: "triage", Description: "Handle incoming issues and routing across the project for long-running triage workflows", LinkName: "matt_skills__triage"},
	}
	model.skillsSelectedIndex = 1

	view := model.renderSkillSelectionView()
	if !strings.Contains(view, "triage") {
		t.Fatalf("expected skill name in view, got %q", view)
	}
	if !strings.Contains(view, "Handle incoming issues") {
		t.Fatalf("expected inline description in view, got %q", view)
	}
	if strings.Contains(view, "Description:") {
		t.Fatalf("did not expect selected skill description footer in view, got %q", view)
	}
}

func TestRenderSkillSourceDetailShowsColumnsWithoutDescriptionFooter(t *testing.T) {
	model := NewModel(fakeProvider{}, nil, "/tmp/work", "default", 80, 12, false)
	model.view = ViewSkillSourceDetail
	model.skillSourceDetailItems = []skillSourceDetailItem{
		{Kind: "update", Label: "Update source"},
		{Kind: "remove", Label: "Remove source"},
		{Kind: "skill", Label: "triage", Description: "Handle incoming issues and routing"},
	}
	model.skillDetailSelectedIndex = 2

	view := model.renderSkillSourceDetail()
	if !strings.Contains(view, "Update source") || !strings.Contains(view, "Remove source") {
		t.Fatalf("expected action rows in view, got %q", view)
	}
	if !strings.Contains(view, "triage") || !strings.Contains(view, "Handle incoming issues") {
		t.Fatalf("expected skill row with inline description in view, got %q", view)
	}
	if strings.Contains(view, "Description:") {
		t.Fatalf("did not expect selected skill description footer in view, got %q", view)
	}
}

func TestSkillSelectionWindowStartCentersSelectionUntilBottom(t *testing.T) {
	tests := []struct {
		name      string
		itemCount int
		selected  int
		rows      int
		wantStart int
	}{
		{name: "top clamps to zero", itemCount: 20, selected: 1, rows: 6, wantStart: 0},
		{name: "middle stays centered", itemCount: 20, selected: 10, rows: 6, wantStart: 7},
		{name: "bottom clamps to final window", itemCount: 20, selected: 19, rows: 6, wantStart: 14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skillSelectionWindowStart(tt.itemCount, tt.selected, tt.rows)
			if got != tt.wantStart {
				t.Fatalf("skillSelectionWindowStart(%d, %d, %d) = %d, want %d", tt.itemCount, tt.selected, tt.rows, got, tt.wantStart)
			}
		})
	}
}
