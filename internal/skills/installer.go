package skills

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type CLIInstaller struct{}

type listedSkill struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (CLIInstaller) Install(source string, tempDir string) ([]PreparedSkill, error) {
	if err := runSkillsCommand(tempDir, "add", source, "-y"); err != nil {
		return nil, err
	}
	listed, err := listInstalledSkills(tempDir)
	if err != nil {
		return nil, err
	}
	var prepared []PreparedSkill
	for _, skill := range listed {
		cleanName := strings.TrimSpace(skill.Name)
		cleanPath := filepath.Clean(strings.TrimSpace(skill.Path))
		if cleanName == "" || cleanPath == "" {
			continue
		}
		prepared = append(prepared, PreparedSkill{Name: cleanName, Path: cleanPath})
	}
	return prepared, nil
}

func runSkillsCommand(workdir string, args ...string) error {
	cmd := exec.Command("npx", append([]string{"--yes", "skills"}, args...)...)
	cmd.Dir = workdir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func listInstalledSkills(workdir string) ([]listedSkill, error) {
	cmd := exec.Command("npx", "--yes", "skills", "list", "--json")
	cmd.Dir = workdir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(stderr.String())
		if output == "" {
			output = strings.TrimSpace(stdout.String())
		}
		if output == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, output)
	}
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, nil
	}
	jsonStart := strings.Index(output, "[")
	if jsonStart >= 0 {
		output = output[jsonStart:]
	}
	var listed []listedSkill
	if err := json.Unmarshal([]byte(output), &listed); err != nil {
		return nil, err
	}
	return listed, nil
}
