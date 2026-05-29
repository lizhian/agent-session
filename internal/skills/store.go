package skills

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lizhian/agent-session/internal/session"
)

var sourcePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

type Catalog struct {
	Sources []SourceRecord `json:"sources"`
}

type SourceRecord struct {
	Source      string        `json:"source"`
	SourceSafe  string        `json:"sourceSafe"`
	InstalledAt string        `json:"installedAt,omitempty"`
	UpdatedAt   string        `json:"updatedAt,omitempty"`
	Skills      []SkillRecord `json:"skills"`
}

type SkillRecord struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	GlobalEnabled bool     `json:"globalEnabled,omitempty"`
	ProjectPaths  []string `json:"projectPaths,omitempty"`
	InstalledAt   string   `json:"installedAt,omitempty"`
	UpdatedAt     string   `json:"updatedAt,omitempty"`
}

type InstalledSkill struct {
	Source     string
	SourceSafe string
	Skill      SkillRecord
}

type PreparedSkill struct {
	Name string
	Path string
}

type Installer interface {
	Install(source string, tempDir string) ([]PreparedSkill, error)
}

type Store struct {
	RootDir         string
	RepositoryDir   string
	CatalogPath     string
	GlobalTargetDir string
	Installer       Installer
	Now             func() time.Time
}

type SourceSyncResult struct {
	SourceSafe     string
	InstalledCount int
	RemovedCount   int
	Updated        bool
}

type RemoveSourceImpact struct {
	InstalledSkills       int
	GlobalEnabledSkills   int
	CurrentProjectSkills  int
	OtherProjectPathCount int
}

func NewStore(rootDir string) *Store {
	home := session.HomeDir()
	if rootDir == "" {
		rootDir = filepath.Join(home, ".agent-session")
	}
	return &Store{
		RootDir:         filepath.Clean(rootDir),
		RepositoryDir:   filepath.Join(filepath.Clean(rootDir), "skills"),
		CatalogPath:     filepath.Join(filepath.Clean(rootDir), "install-skills.json"),
		GlobalTargetDir: filepath.Join(home, ".agents", "skills"),
		Installer:       CLIInstaller{},
		Now:             time.Now,
	}
}

func ValidateSource(source string) error {
	if !sourcePattern.MatchString(strings.TrimSpace(source)) {
		return fmt.Errorf("invalid skill source %q: expected owner/repo", source)
	}
	return nil
}

func SourceSafeName(source string) string {
	return strings.ReplaceAll(strings.TrimSpace(source), "/", "_")
}

func ActivationLinkName(sourceSafe, skillName string) string {
	return sourceSafe + "__" + skillName
}

func (s *Store) ProjectTargetDir(cwd string) (string, error) {
	projectPath, err := canonicalProjectPath(cwd)
	if err != nil {
		return "", err
	}
	return filepath.Join(projectPath, ".agents", "skills"), nil
}

func (s *Store) LoadCatalog() (Catalog, error) {
	data, err := os.ReadFile(s.CatalogPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Catalog{}, nil
		}
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, err
	}
	normalizeCatalog(&catalog)
	return catalog, nil
}

func (s *Store) SaveCatalog(catalog Catalog) error {
	normalizeCatalog(&catalog)
	if err := os.MkdirAll(filepath.Dir(s.CatalogPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.CatalogPath, data, 0o600)
}

func (s *Store) ListInstalledSkills() ([]InstalledSkill, error) {
	catalog, err := s.LoadCatalog()
	if err != nil {
		return nil, err
	}
	var installed []InstalledSkill
	for _, source := range catalog.Sources {
		for _, skill := range source.Skills {
			installed = append(installed, InstalledSkill{Source: source.Source, SourceSafe: source.SourceSafe, Skill: skill})
		}
	}
	return installed, nil
}

func (s *Store) InstallOrUpdateSource(source string) (SourceSyncResult, error) {
	if err := ValidateSource(source); err != nil {
		return SourceSyncResult{}, err
	}
	source = strings.TrimSpace(source)
	sourceSafe := SourceSafeName(source)
	tempDir, err := os.MkdirTemp("", "agent-session-skills-")
	if err != nil {
		return SourceSyncResult{}, err
	}
	defer os.RemoveAll(tempDir)
	prepared, err := s.Installer.Install(source, tempDir)
	if err != nil {
		return SourceSyncResult{}, err
	}
	if len(prepared) == 0 {
		return SourceSyncResult{}, fmt.Errorf("no skills were installed from %s", source)
	}
	return s.commitPreparedSource(source, sourceSafe, prepared)
}

func (s *Store) RemoveSource(sourceSafe string) (SourceSyncResult, error) {
	catalog, err := s.LoadCatalog()
	if err != nil {
		return SourceSyncResult{}, err
	}
	idx := catalog.SourceIndex(sourceSafe)
	if idx < 0 {
		return SourceSyncResult{}, fmt.Errorf("unknown skill source %s", sourceSafe)
	}
	oldCatalog := cloneCatalog(catalog)
	oldSource := catalog.Sources[idx]
	newCatalog := cloneCatalog(catalog)
	newCatalog.Sources = append(newCatalog.Sources[:idx], newCatalog.Sources[idx+1:]...)
	projectPaths := unionProjectPaths(oldCatalog, newCatalog)
	managed := unionManagedLinkNames(oldCatalog, newCatalog)
	finalDir := filepath.Join(s.RepositoryDir, sourceSafe)
	backupDir := finalDir + ".bak"
	_ = os.RemoveAll(backupDir)
	if session.DirExists(finalDir) {
		if err := os.Rename(finalDir, backupDir); err != nil {
			return SourceSyncResult{}, err
		}
	}
	if err := s.SaveCatalog(newCatalog); err != nil {
		if session.DirExists(backupDir) {
			_ = os.Rename(backupDir, finalDir)
		}
		return SourceSyncResult{}, err
	}
	if err := s.syncCatalogScopes(newCatalog, projectPaths, managed); err != nil {
		_ = s.SaveCatalog(oldCatalog)
		if session.DirExists(finalDir) {
			_ = os.RemoveAll(finalDir)
		}
		if session.DirExists(backupDir) {
			_ = os.Rename(backupDir, finalDir)
		}
		_ = s.syncCatalogScopes(oldCatalog, projectPaths, managed)
		return SourceSyncResult{}, err
	}
	if session.DirExists(backupDir) {
		_ = os.RemoveAll(backupDir)
	}
	return SourceSyncResult{SourceSafe: sourceSafe, InstalledCount: 0, RemovedCount: len(oldSource.Skills), Updated: false}, nil
}

func (s *Store) RemoveSourceImpact(sourceSafe string, currentProject string) (RemoveSourceImpact, error) {
	catalog, err := s.LoadCatalog()
	if err != nil {
		return RemoveSourceImpact{}, err
	}
	idx := catalog.SourceIndex(sourceSafe)
	if idx < 0 {
		return RemoveSourceImpact{}, fmt.Errorf("unknown skill source %s", sourceSafe)
	}
	var impact RemoveSourceImpact
	impact.InstalledSkills = len(catalog.Sources[idx].Skills)
	projectPath, err := canonicalProjectPath(currentProject)
	if err != nil {
		projectPath = ""
	}
	otherProjects := make(map[string]struct{})
	for _, skill := range catalog.Sources[idx].Skills {
		if skill.GlobalEnabled {
			impact.GlobalEnabledSkills++
		}
		for _, path := range skill.ProjectPaths {
			if path == projectPath {
				impact.CurrentProjectSkills++
				continue
			}
			otherProjects[path] = struct{}{}
		}
	}
	impact.OtherProjectPathCount = len(otherProjects)
	return impact, nil
}

func (s *Store) ApplyGlobalSelection(selected map[string]bool) error {
	catalog, err := s.LoadCatalog()
	if err != nil {
		return err
	}
	oldCatalog := cloneCatalog(catalog)
	for si := range catalog.Sources {
		for ki := range catalog.Sources[si].Skills {
			key := ActivationLinkName(catalog.Sources[si].SourceSafe, catalog.Sources[si].Skills[ki].Name)
			catalog.Sources[si].Skills[ki].GlobalEnabled = selected[key]
		}
	}
	if err := s.SaveCatalog(catalog); err != nil {
		return err
	}
	managed := managedLinkNames(catalog)
	if err := s.syncGlobalScope(catalog, managed); err != nil {
		_ = s.SaveCatalog(oldCatalog)
		_ = s.syncGlobalScope(oldCatalog, unionManagedLinkNames(oldCatalog, catalog))
		return err
	}
	return nil
}

func (s *Store) ApplyProjectSelection(cwd string, selected map[string]bool) error {
	projectPath, err := canonicalProjectPath(cwd)
	if err != nil {
		return err
	}
	catalog, err := s.LoadCatalog()
	if err != nil {
		return err
	}
	oldCatalog := cloneCatalog(catalog)
	for si := range catalog.Sources {
		for ki := range catalog.Sources[si].Skills {
			key := ActivationLinkName(catalog.Sources[si].SourceSafe, catalog.Sources[si].Skills[ki].Name)
			enabled := selected[key]
			catalog.Sources[si].Skills[ki].ProjectPaths = setProjectPath(catalog.Sources[si].Skills[ki].ProjectPaths, projectPath, enabled)
		}
	}
	if err := s.SaveCatalog(catalog); err != nil {
		return err
	}
	managed := managedLinkNames(catalog)
	if err := s.syncProjectScope(catalog, projectPath, managed); err != nil {
		_ = s.SaveCatalog(oldCatalog)
		_ = s.syncProjectScope(oldCatalog, projectPath, unionManagedLinkNames(oldCatalog, catalog))
		return err
	}
	return nil
}

func (c Catalog) SourceIndex(sourceSafe string) int {
	for i, source := range c.Sources {
		if source.SourceSafe == sourceSafe {
			return i
		}
	}
	return -1
}

func (c Catalog) SourceBySafe(sourceSafe string) *SourceRecord {
	idx := c.SourceIndex(sourceSafe)
	if idx < 0 {
		return nil
	}
	return &c.Sources[idx]
}

func (s *Store) commitPreparedSource(source string, sourceSafe string, prepared []PreparedSkill) (SourceSyncResult, error) {
	oldCatalog, err := s.LoadCatalog()
	if err != nil {
		return SourceSyncResult{}, err
	}
	oldIdx := oldCatalog.SourceIndex(sourceSafe)
	updated := oldIdx >= 0
	var oldSource SourceRecord
	if updated {
		oldSource = cloneSourceRecord(oldCatalog.Sources[oldIdx])
	}
	now := providerTimestamp(s.Now())
	newSource := SourceRecord{Source: source, SourceSafe: sourceSafe, InstalledAt: now, UpdatedAt: now}
	if updated {
		newSource.InstalledAt = oldSource.InstalledAt
	}
	prepared = sortedPreparedSkills(prepared)
	for _, skill := range prepared {
		record := SkillRecord{Name: skill.Name, Description: readInstalledSkillDescription(skill.Path), InstalledAt: now, UpdatedAt: now}
		if previous, ok := oldSource.skillByName(skill.Name); ok {
			record.GlobalEnabled = previous.GlobalEnabled
			record.ProjectPaths = append([]string(nil), previous.ProjectPaths...)
			record.InstalledAt = previous.InstalledAt
		}
		newSource.Skills = append(newSource.Skills, record)
	}
	stagingDir := filepath.Join(s.RepositoryDir, ".staging-"+sourceSafe)
	_ = os.RemoveAll(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return SourceSyncResult{}, err
	}
	finalDir := filepath.Join(s.RepositoryDir, sourceSafe)
	for _, skill := range prepared {
		if err := copyDir(skill.Path, filepath.Join(stagingDir, skill.Name)); err != nil {
			_ = os.RemoveAll(stagingDir)
			return SourceSyncResult{}, err
		}
	}
	newCatalog := cloneCatalog(oldCatalog)
	if oldIdx >= 0 {
		newCatalog.Sources[oldIdx] = newSource
	} else {
		newCatalog.Sources = append(newCatalog.Sources, newSource)
	}
	projectPaths := unionProjectPaths(oldCatalog, newCatalog)
	managed := unionManagedLinkNames(oldCatalog, newCatalog)
	backupDir := finalDir + ".bak"
	_ = os.RemoveAll(backupDir)
	if session.DirExists(finalDir) {
		if err := os.Rename(finalDir, backupDir); err != nil {
			_ = os.RemoveAll(stagingDir)
			return SourceSyncResult{}, err
		}
	}
	if err := os.Rename(stagingDir, finalDir); err != nil {
		if session.DirExists(backupDir) {
			_ = os.Rename(backupDir, finalDir)
		}
		_ = os.RemoveAll(stagingDir)
		return SourceSyncResult{}, err
	}
	if err := s.SaveCatalog(newCatalog); err != nil {
		_ = os.RemoveAll(finalDir)
		if session.DirExists(backupDir) {
			_ = os.Rename(backupDir, finalDir)
		}
		return SourceSyncResult{}, err
	}
	if err := s.syncCatalogScopes(newCatalog, projectPaths, managed); err != nil {
		_ = s.SaveCatalog(oldCatalog)
		if session.DirExists(finalDir) {
			_ = os.RemoveAll(finalDir)
		}
		if session.DirExists(backupDir) {
			_ = os.Rename(backupDir, finalDir)
		}
		_ = s.syncCatalogScopes(oldCatalog, projectPaths, managed)
		return SourceSyncResult{}, err
	}
	if session.DirExists(backupDir) {
		_ = os.RemoveAll(backupDir)
	}
	removedCount := 0
	if updated {
		newNames := make(map[string]struct{}, len(newSource.Skills))
		for _, skill := range newSource.Skills {
			newNames[skill.Name] = struct{}{}
		}
		for _, skill := range oldSource.Skills {
			if _, ok := newNames[skill.Name]; !ok {
				removedCount++
			}
		}
	}
	return SourceSyncResult{SourceSafe: sourceSafe, InstalledCount: len(newSource.Skills), RemovedCount: removedCount, Updated: updated}, nil
}

func (s *Store) syncCatalogScopes(catalog Catalog, projectPaths []string, managed map[string]struct{}) error {
	if err := s.syncGlobalScope(catalog, managed); err != nil {
		return err
	}
	for _, projectPath := range projectPaths {
		if err := s.syncProjectScope(catalog, projectPath, managed); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) syncGlobalScope(catalog Catalog, managed map[string]struct{}) error {
	targetDir := s.GlobalTargetDir
	desired := make(map[string]string)
	for _, source := range catalog.Sources {
		for _, skill := range source.Skills {
			if skill.GlobalEnabled {
				name := ActivationLinkName(source.SourceSafe, skill.Name)
				desired[name] = filepath.Join(s.RepositoryDir, source.SourceSafe, skill.Name)
			}
		}
	}
	return syncTargetDir(targetDir, desired, managed)
}

func (s *Store) syncProjectScope(catalog Catalog, projectPath string, managed map[string]struct{}) error {
	targetDir := filepath.Join(projectPath, ".agents", "skills")
	desired := make(map[string]string)
	for _, source := range catalog.Sources {
		for _, skill := range source.Skills {
			if containsString(skill.ProjectPaths, projectPath) {
				name := ActivationLinkName(source.SourceSafe, skill.Name)
				desired[name] = filepath.Join(s.RepositoryDir, source.SourceSafe, skill.Name)
			}
		}
	}
	return syncTargetDir(targetDir, desired, managed)
}

func managedLinkNames(catalog Catalog) map[string]struct{} {
	managed := make(map[string]struct{})
	for _, source := range catalog.Sources {
		for _, skill := range source.Skills {
			managed[ActivationLinkName(source.SourceSafe, skill.Name)] = struct{}{}
		}
	}
	return managed
}

func unionManagedLinkNames(catalogs ...Catalog) map[string]struct{} {
	managed := make(map[string]struct{})
	for _, catalog := range catalogs {
		for name := range managedLinkNames(catalog) {
			managed[name] = struct{}{}
		}
	}
	return managed
}

func syncTargetDir(targetDir string, desired map[string]string, managed map[string]struct{}) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if _, ok := managed[name]; !ok {
			continue
		}
		if _, keep := desired[name]; keep {
			continue
		}
		if err := os.RemoveAll(filepath.Join(targetDir, name)); err != nil {
			return err
		}
	}
	for name, target := range desired {
		linkPath := filepath.Join(targetDir, name)
		info, err := os.Lstat(linkPath)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				existingTarget, readErr := os.Readlink(linkPath)
				if readErr == nil && existingTarget == target {
					continue
				}
				if err := os.Remove(linkPath); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("managed skill target %s already exists and is not a symlink", linkPath)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Symlink(target, linkPath); err != nil {
			return err
		}
	}
	return nil
}

func normalizeCatalog(catalog *Catalog) {
	for si := range catalog.Sources {
		source := &catalog.Sources[si]
		source.Source = strings.TrimSpace(source.Source)
		source.SourceSafe = strings.TrimSpace(source.SourceSafe)
		for ki := range source.Skills {
			skill := &source.Skills[ki]
			skill.Name = strings.TrimSpace(skill.Name)
			skill.Description = strings.TrimSpace(skill.Description)
			skill.ProjectPaths = uniqueSortedStrings(skill.ProjectPaths)
		}
		sort.Slice(source.Skills, func(i, j int) bool {
			return source.Skills[i].Name < source.Skills[j].Name
		})
	}
	sort.Slice(catalog.Sources, func(i, j int) bool {
		return catalog.Sources[i].Source < catalog.Sources[j].Source
	})
}

func cloneCatalog(catalog Catalog) Catalog {
	clone := Catalog{Sources: make([]SourceRecord, len(catalog.Sources))}
	for i, source := range catalog.Sources {
		clone.Sources[i] = cloneSourceRecord(source)
	}
	return clone
}

func cloneSourceRecord(source SourceRecord) SourceRecord {
	clone := SourceRecord{
		Source:      source.Source,
		SourceSafe:  source.SourceSafe,
		InstalledAt: source.InstalledAt,
		UpdatedAt:   source.UpdatedAt,
		Skills:      make([]SkillRecord, len(source.Skills)),
	}
	for i, skill := range source.Skills {
		clone.Skills[i] = SkillRecord{
			Name:          skill.Name,
			Description:   skill.Description,
			GlobalEnabled: skill.GlobalEnabled,
			ProjectPaths:  append([]string(nil), skill.ProjectPaths...),
			InstalledAt:   skill.InstalledAt,
			UpdatedAt:     skill.UpdatedAt,
		}
	}
	return clone
}

func (s SourceRecord) skillByName(name string) (SkillRecord, bool) {
	for _, skill := range s.Skills {
		if skill.Name == name {
			return skill, true
		}
	}
	return SkillRecord{}, false
}

func sortedPreparedSkills(prepared []PreparedSkill) []PreparedSkill {
	clone := append([]PreparedSkill(nil), prepared...)
	sort.Slice(clone, func(i, j int) bool {
		return clone[i].Name < clone[j].Name
	})
	return clone
}

func readInstalledSkillDescription(skillDir string) string {
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return ""
	}
	return parseInstalledSkillDescription(string(data))
}

func parseInstalledSkillDescription(content string) string {
	content = strings.TrimPrefix(content, "\ufeff")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			break
		}
		if !strings.HasPrefix(line, "description:") {
			continue
		}
		value := normalizeInstalledSkillDescription(strings.TrimSpace(strings.TrimPrefix(line, "description:")))
		if value == ">" || value == "|" {
			return ""
		}
		return value
	}
	return ""
}

func normalizeInstalledSkillDescription(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, `"`) {
		if !strings.HasSuffix(value, `"`) || len(value) < 2 {
			return ""
		}
		value = value[1 : len(value)-1]
	} else if strings.HasPrefix(value, `'`) {
		if !strings.HasSuffix(value, `'`) || len(value) < 2 {
			return ""
		}
		value = value[1 : len(value)-1]
	}
	return strings.TrimSpace(value)
}

func providerTimestamp(now time.Time) string {
	if now.IsZero() {
		return ""
	}
	return now.Format(time.RFC3339)
}

func unionProjectPaths(catalogs ...Catalog) []string {
	seen := make(map[string]struct{})
	for _, catalog := range catalogs {
		for _, source := range catalog.Sources {
			for _, skill := range source.Skills {
				for _, projectPath := range skill.ProjectPaths {
					seen[projectPath] = struct{}{}
				}
			}
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func setProjectPath(paths []string, target string, enabled bool) []string {
	var filtered []string
	for _, path := range paths {
		if path != target {
			filtered = append(filtered, path)
		}
	}
	if enabled {
		filtered = append(filtered, target)
	}
	return uniqueSortedStrings(filtered)
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func canonicalProjectPath(cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return "", fmt.Errorf("project path is empty")
	}
	return filepath.Abs(cwd)
}

func copyDir(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source path %s is not a directory", src)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if entryInfo.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath, entryInfo.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src string, dst string, perm os.FileMode) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer to.Close()
	if _, err := io.Copy(to, from); err != nil {
		return err
	}
	return nil
}
