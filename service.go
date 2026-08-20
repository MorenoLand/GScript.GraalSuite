package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type AppService struct {
	app         *application.App
	window      *application.WebviewWindow
	mu          sync.RWMutex
	index       map[string]string
	lastDir     string
	rescanDone  bool
	scanRunning bool
	openFileArg string
	exeDir      string
	version     string
}

func NewAppService(version string) *AppService {
	return &AppService{index: make(map[string]string), openFileArg: firstExistingPath(os.Args[1:]), exeDir: executableDirectory(), version: version}
}

func (s *AppService) ResolvePath(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index[strings.ToLower(name)]
}

func (s *AppService) GetOpenFileArg() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.openFileArg
}

func (s *AppService) ReadDir(path string) ([]string, error) {
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(s.exeDir, resolved)
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func (s *AppService) ReadTextFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func (s *AppService) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (s *AppService) WriteTextFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func (s *AppService) WriteFile(path, encoded string) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *AppService) CheckForUpdate() (string, error) {
	if s.app == nil || s.app.Updater == nil {
		return "", fmt.Errorf("updater is not configured")
	}
	release, err := s.app.Updater.Check(context.Background())
	if err != nil || release == nil {
		return "", err
	}
	return release.Version, nil
}

func (s *AppService) DoUpdate() error {
	if s.app == nil || s.app.Updater == nil {
		return fmt.Errorf("updater is not configured")
	}
	ctx := context.Background()
	if _, err := s.app.Updater.Check(ctx); err != nil {
		return err
	}
	if err := s.app.Updater.DownloadAndInstall(ctx); err != nil {
		return err
	}
	return s.app.Updater.Restart(ctx)
}

func (s *AppService) RegisterFileAssociations() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("file associations are only supported on Windows")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	associations := []struct {
		ext  string
		desc string
	}{
		{"nw", "Graal Level"},
		{"gmap", "Graal Map"},
		{"gani", "Graal Animation"},
		{"zelda", "Graal Level (Zelda)"},
		{"graal", "Graal Level"},
	}
	failed := make([]string, 0)
	for _, association := range associations {
		progID := "GSuite." + association.ext
		if err := addRegistryValue(`HKCU\Software\Classes\.`+association.ext, "", progID); err != nil {
			failed = append(failed, association.ext)
			continue
		}
		if err := addRegistryValue(`HKCU\Software\Classes\`+progID, "", association.desc); err != nil {
			failed = append(failed, association.ext)
			continue
		}
		command := fmt.Sprintf(`"%s" "%%1"`, exe)
		if err := addRegistryValue(`HKCU\Software\Classes\`+progID+`\shell\open\command`, "", command); err != nil {
			failed = append(failed, association.ext)
		}
	}
	if len(failed) != 0 {
		return "", fmt.Errorf("failed: %s", strings.Join(failed, ", "))
	}
	return "Registered .nw .gmap .gani .zelda .graal", nil
}

func addRegistryValue(key, name, value string) error {
	return exec.Command("reg", "add", key, "/ve", "/d", value, "/f").Run()
}

func (s *AppService) emit(name string, data any) {
	if s.app != nil {
		s.app.Event.Emit(name, data)
	}
}

func firstExistingPath(args []string) string {
	return firstExistingPathInDir(args, "")
}

func firstExistingPathInDir(args []string, workingDir string) string {
	executable, _ := os.Executable()
	executable, _ = filepath.Abs(executable)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		candidate := arg
		if workingDir != "" && !filepath.IsAbs(candidate) {
			candidate = filepath.Join(workingDir, candidate)
		}
		absolute, err := filepath.Abs(candidate)
		if err == nil && executable != "" && samePath(absolute, executable) {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
	}
	return ""
}

func samePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func executableDirectory() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
