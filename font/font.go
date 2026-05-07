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

// LoadFirstWindowsFont 依次尝试多个候选名，返回第一个成功加载的系统字体。
func LoadFirstWindowsFont(names ...string) ([]byte, error) {
	var lastErr error
	for _, name := range names {
		data, err := LoadWindowsFont(name)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("font candidates are empty")
	}
	return nil, lastErr
}

func resolveWindowsFontPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("font name is empty")
	}

	if filepath.IsAbs(name) {
		return filepath.Clean(name), nil
	}

	fontDirs, err := windowsFontsDirs()
	if err != nil {
		return "", err
	}

	for _, fontDir := range fontDirs {
		for _, candidate := range fontNameCandidates(name) {
			path := filepath.Join(fontDir, candidate)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	targetName := strings.ToLower(name)
	targetBase := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	for _, fontDir := range fontDirs {
		entries, err := os.ReadDir(fontDir)
		if err != nil {
			return "", fmt.Errorf("read windows fonts dir %q: %w", fontDir, err)
		}

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
	}

	return "", fmt.Errorf("windows font %q not found in %q", name, strings.Join(fontDirs, ", "))
}

func windowsFontsDirs() ([]string, error) {
	winDir := strings.TrimSpace(os.Getenv("WINDIR"))
	if winDir == "" {
		winDir = `C:\Windows`
	}

	var dirs []string
	seen := map[string]struct{}{}

	addDir := func(dir string) error {
		if strings.TrimSpace(dir) == "" {
			return nil
		}
		dir = filepath.Clean(dir)
		if _, ok := seen[dir]; ok {
			return nil
		}

		info, err := os.Stat(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("stat windows fonts dir %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("windows fonts dir %q is not a directory", dir)
		}

		seen[dir] = struct{}{}
		dirs = append(dirs, dir)
		return nil
	}

	if err := addDir(filepath.Join(winDir, "Fonts")); err != nil {
		return nil, err
	}

	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData != "" {
		if err := addDir(filepath.Join(localAppData, "Microsoft", "Windows", "Fonts")); err != nil {
			return nil, err
		}
	}

	if len(dirs) == 0 {
		return nil, errors.New("no windows fonts directory found")
	}
	return dirs, nil
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
