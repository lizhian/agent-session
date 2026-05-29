package skills

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeInstaller struct {
	prepared []PreparedSkill
	err      error
}

func (f fakeInstaller) Install(source string, tempDir string) ([]PreparedSkill, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.prepared, nil
}

func TestSourceSafeName(t *testing.T) {
	if got, want := SourceSafeName("mattpocock/skills"), "mattpocock_skills"; got != want {
		t.Fatalf("SourceSafeName() = %q, want %q", got, want)
	}
}

func TestValidateSource(t *testing.T) {
	if err := ValidateSource("mattpocock/skills"); err != nil {
		t.Fatalf("ValidateSource valid error = %v", err)
	}
	if err := ValidateSource("https://github.com/mattpocock/skills"); err == nil {
		t.Fatal("expected invalid URL source to fail validation")
	}
}

func TestInstallOrUpdateSourceAndScopeSync(t *testing.T) {
	rootDir := t.TempDir()
	globalDir := filepath.Join(rootDir, "global-skills")
	repoDir := filepath.Join(rootDir, "repo")
	projectDir := filepath.Join(rootDir, "project")
	preparedRoot := filepath.Join(rootDir, "prepared")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(preparedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	triageDir := createSkillDir(t, preparedRoot, "triage")
	zoomOutDir := createSkillDir(t, preparedRoot, "zoom-out")
	store := &Store{
		RootDir:         rootDir,
		RepositoryDir:   repoDir,
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: globalDir,
		Installer: fakeInstaller{prepared: []PreparedSkill{
			{Name: "triage", Path: triageDir},
			{Name: "zoom-out", Path: zoomOutDir},
		}},
		Now: func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}

	result, err := store.InstallOrUpdateSource("mattpocock/skills")
	if err != nil {
		t.Fatal(err)
	}
	if result.InstalledCount != 2 {
		t.Fatalf("InstalledCount = %d, want 2", result.InstalledCount)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "mattpocock_skills", "triage", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	selectedGlobal := map[string]bool{"mattpocock_skills__triage": true}
	if err := store.ApplyGlobalSelection(selectedGlobal); err != nil {
		t.Fatal(err)
	}
	assertSymlinkTarget(t, filepath.Join(globalDir, "mattpocock_skills__triage"), filepath.Join(repoDir, "mattpocock_skills", "triage"))

	selectedProject := map[string]bool{"mattpocock_skills__zoom-out": true}
	if err := store.ApplyProjectSelection(projectDir, selectedProject); err != nil {
		t.Fatal(err)
	}
	assertSymlinkTarget(t, filepath.Join(projectDir, ".agents", "skills", "mattpocock_skills__zoom-out"), filepath.Join(repoDir, "mattpocock_skills", "zoom-out"))

	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	source := catalog.SourceBySafe("mattpocock_skills")
	if source == nil {
		t.Fatal("expected source mattpocock_skills in catalog")
	}
	if len(source.Skills) != 2 {
		t.Fatalf("skills len = %d, want 2", len(source.Skills))
	}
	if !source.Skills[0].GlobalEnabled && !source.Skills[1].GlobalEnabled {
		t.Fatal("expected one skill to be globally enabled")
	}
}

func TestUpdateSourceMirrorsRemovedSkillsAndCleansScopes(t *testing.T) {
	rootDir := t.TempDir()
	globalDir := filepath.Join(rootDir, "global-skills")
	repoDir := filepath.Join(rootDir, "repo")
	projectDir := filepath.Join(rootDir, "project")
	preparedRoot := filepath.Join(rootDir, "prepared")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(preparedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	triageDir := createSkillDir(t, preparedRoot, "triage")
	zoomOutDir := createSkillDir(t, preparedRoot, "zoom-out")
	store := &Store{
		RootDir:         rootDir,
		RepositoryDir:   repoDir,
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: globalDir,
		Installer:       fakeInstaller{prepared: []PreparedSkill{{Name: "triage", Path: triageDir}, {Name: "zoom-out", Path: zoomOutDir}}},
		Now:             func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}
	if _, err := store.InstallOrUpdateSource("mattpocock/skills"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyGlobalSelection(map[string]bool{"mattpocock_skills__triage": true}); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyProjectSelection(projectDir, map[string]bool{"mattpocock_skills__zoom-out": true}); err != nil {
		t.Fatal(err)
	}
	store.Installer = fakeInstaller{prepared: []PreparedSkill{{Name: "triage", Path: triageDir}}}
	result, err := store.InstallOrUpdateSource("mattpocock/skills")
	if err != nil {
		t.Fatal(err)
	}
	if result.RemovedCount != 1 {
		t.Fatalf("RemovedCount = %d, want 1", result.RemovedCount)
	}
	if _, err := os.Lstat(filepath.Join(projectDir, ".agents", "skills", "mattpocock_skills__zoom-out")); !os.IsNotExist(err) {
		t.Fatalf("expected removed project symlink, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "mattpocock_skills", "zoom-out")); !os.IsNotExist(err) {
		t.Fatalf("expected removed mirrored skill, got err=%v", err)
	}
	assertSymlinkTarget(t, filepath.Join(globalDir, "mattpocock_skills__triage"), filepath.Join(repoDir, "mattpocock_skills", "triage"))
}

func TestInstallOrUpdateSourceStoresDescriptionFromFrontmatter(t *testing.T) {
	rootDir := t.TempDir()
	preparedRoot := filepath.Join(rootDir, "prepared")
	if err := os.MkdirAll(preparedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	triageDir := createSkillDirWithBody(t, preparedRoot, "triage", "---\ndescription: \"Handle incoming issues\"\n---\n# triage\n")
	store := &Store{
		RootDir:         rootDir,
		RepositoryDir:   filepath.Join(rootDir, "repo"),
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: filepath.Join(rootDir, "global-skills"),
		Installer:       fakeInstaller{prepared: []PreparedSkill{{Name: "triage", Path: triageDir}}},
		Now:             func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}

	if _, err := store.InstallOrUpdateSource("mattpocock/skills"); err != nil {
		t.Fatal(err)
	}

	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	source := catalog.SourceBySafe("mattpocock_skills")
	if source == nil {
		t.Fatal("expected source mattpocock_skills in catalog")
	}
	if got := source.Skills[0].Description; got != "Handle incoming issues" {
		t.Fatalf("description = %q, want %q", got, "Handle incoming issues")
	}
}

func TestUpdateSourceRefreshesDescription(t *testing.T) {
	rootDir := t.TempDir()
	preparedRoot := filepath.Join(rootDir, "prepared")
	if err := os.MkdirAll(preparedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	triageV1Dir := createSkillDirWithBody(t, preparedRoot, "triage-v1", "---\ndescription: old description\n---\n# triage\n")
	triageV2Dir := createSkillDirWithBody(t, preparedRoot, "triage-v2", "---\ndescription: new description\n---\n# triage\n")
	store := &Store{
		RootDir:         rootDir,
		RepositoryDir:   filepath.Join(rootDir, "repo"),
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: filepath.Join(rootDir, "global-skills"),
		Installer:       fakeInstaller{prepared: []PreparedSkill{{Name: "triage", Path: triageV1Dir}}},
		Now:             func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}

	if _, err := store.InstallOrUpdateSource("mattpocock/skills"); err != nil {
		t.Fatal(err)
	}
	store.Installer = fakeInstaller{prepared: []PreparedSkill{{Name: "triage", Path: triageV2Dir}}}
	if _, err := store.InstallOrUpdateSource("mattpocock/skills"); err != nil {
		t.Fatal(err)
	}

	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	source := catalog.SourceBySafe("mattpocock_skills")
	if source == nil {
		t.Fatal("expected source mattpocock_skills in catalog")
	}
	if got := source.Skills[0].Description; got != "new description" {
		t.Fatalf("description = %q, want %q", got, "new description")
	}
}

func TestInstallOrUpdateSourceAllowsMissingDescription(t *testing.T) {
	rootDir := t.TempDir()
	preparedRoot := filepath.Join(rootDir, "prepared")
	if err := os.MkdirAll(preparedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	triageDir := createSkillDirWithBody(t, preparedRoot, "triage", "---\nname: triage\n---\n# triage\n")
	store := &Store{
		RootDir:         rootDir,
		RepositoryDir:   filepath.Join(rootDir, "repo"),
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: filepath.Join(rootDir, "global-skills"),
		Installer:       fakeInstaller{prepared: []PreparedSkill{{Name: "triage", Path: triageDir}}},
		Now:             func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}

	if _, err := store.InstallOrUpdateSource("mattpocock/skills"); err != nil {
		t.Fatal(err)
	}

	catalog, err := store.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	source := catalog.SourceBySafe("mattpocock_skills")
	if source == nil {
		t.Fatal("expected source mattpocock_skills in catalog")
	}
	if got := source.Skills[0].Description; got != "" {
		t.Fatalf("description = %q, want empty", got)
	}
}

func TestRemoveSourceImpactAndRemoveSource(t *testing.T) {
	rootDir := t.TempDir()
	globalDir := filepath.Join(rootDir, "global-skills")
	repoDir := filepath.Join(rootDir, "repo")
	projectDir := filepath.Join(rootDir, "project")
	otherProject := filepath.Join(rootDir, "other-project")
	preparedRoot := filepath.Join(rootDir, "prepared")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherProject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(preparedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	triageDir := createSkillDir(t, preparedRoot, "triage")
	store := &Store{
		RootDir:         rootDir,
		RepositoryDir:   repoDir,
		CatalogPath:     filepath.Join(rootDir, "install-skills.json"),
		GlobalTargetDir: globalDir,
		Installer:       fakeInstaller{prepared: []PreparedSkill{{Name: "triage", Path: triageDir}}},
		Now:             func() time.Time { return time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC) },
	}
	if _, err := store.InstallOrUpdateSource("mattpocock/skills"); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyGlobalSelection(map[string]bool{"mattpocock_skills__triage": true}); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyProjectSelection(projectDir, map[string]bool{"mattpocock_skills__triage": true}); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyProjectSelection(otherProject, map[string]bool{"mattpocock_skills__triage": true}); err != nil {
		t.Fatal(err)
	}
	impact, err := store.RemoveSourceImpact("mattpocock_skills", projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if impact.InstalledSkills != 1 || impact.GlobalEnabledSkills != 1 || impact.CurrentProjectSkills != 1 || impact.OtherProjectPathCount != 1 {
		t.Fatalf("unexpected impact: %+v", impact)
	}
	if _, err := store.RemoveSource("mattpocock_skills"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "mattpocock_skills")); !os.IsNotExist(err) {
		t.Fatalf("expected source dir removed, got err=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(globalDir, "mattpocock_skills__triage")); !os.IsNotExist(err) {
		t.Fatalf("expected global symlink removed, got err=%v", err)
	}
}

func createSkillDir(t *testing.T, root string, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createSkillDirWithBody(t *testing.T, root string, name string, body string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func assertSymlinkTarget(t *testing.T, linkPath string, target string) {
	t.Helper()
	got, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("symlink target = %q, want %q", got, target)
	}
}
