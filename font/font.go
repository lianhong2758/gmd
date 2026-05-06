package font

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var windowsFontExtensions = []string{".ttf", ".ttc", ".otf"}

// LoadWindowsFont 从 Windows 系统字体目录加载字体文件。
//
// name 支持传入完整文件名，例如 "msyh.ttc"，也支持不带扩展名的名称，
// 例如 "msyh"。如果传入绝对路径，则直接读取该路径。
func LoadWindowsFont(name string) ([]byte, error) {
	path, err := resolveWindowsFontPath(name)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read windows font %q: %w", path, err)
	}
	return data, nil
}

func resolveWindowsFontPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("font name is empty")
	}

	if filepath.IsAbs(name) {
		return filepath.Clean(name), nil
	}

	fontDir, err := windowsFontsDir()
	if err != nil {
		return "", err
	}

	for _, candidate := range fontNameCandidates(name) {
		path := filepath.Join(fontDir, candidate)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	entries, err := os.ReadDir(fontDir)
	if err != nil {
		return "", fmt.Errorf("read windows fonts dir %q: %w", fontDir, err)
	}

	targetName := strings.ToLower(name)
	targetBase := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		entryName := entry.Name()
		if strings.ToLower(entryName) == targetName {
			return filepath.Join(fontDir, entryName), nil
		}
		if strings.ToLower(strings.TrimSuffix(entryName, filepath.Ext(entryName))) == targetBase {
			return filepath.Join(fontDir, entryName), nil
		}
	}

	return "", fmt.Errorf("windows font %q not found in %q", name, fontDir)
}

func windowsFontsDir() (string, error) {
	winDir := strings.TrimSpace(os.Getenv("WINDIR"))
	if winDir == "" {
		winDir = `C:\Windows`
	}

	fontDir := filepath.Join(winDir, "Fonts")
	info, err := os.Stat(fontDir)
	if err != nil {
		return "", fmt.Errorf("stat windows fonts dir %q: %w", fontDir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("windows fonts dir %q is not a directory", fontDir)
	}
	return fontDir, nil
}

func fontNameCandidates(name string) []string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != "" {
		return []string{name}
	}

	candidates := make([]string, 0, len(windowsFontExtensions))
	for _, suffix := range windowsFontExtensions {
		candidates = append(candidates, name+suffix)
	}
	return candidates
}
