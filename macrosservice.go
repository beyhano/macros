package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FileEntry represents a file or directory in the Macros tree.
type FileEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Path  string `json:"path"`
}

// VersionInfo describes a single saved version.
type VersionInfo struct {
	VersionID  string `json:"versionId"`
	ModifiedAt string `json:"modifiedAt"`
	Size       int64  `json:"size"`
	SHA1       string `json:"sha1"`
}

// FileMeta is a metadata.json entry.
type FileMeta struct {
	Name         string      `json:"Name"`
	Description  string      `json:"Description"`
	Author       string      `json:"Author"`
	Era          string      `json:"Era,omitempty"`
	Shard        string      `json:"Shard,omitempty"`
	ID           string      `json:"Id"`
	Categories   interface{} `json:"Categories"`
	FileName     string      `json:"FileName"`
	Size         int         `json:"Size"`
	SHA1         string      `json:"SHA1"`
	ModifiedDate string      `json:"ModifiedDate"`
}

// MacrosService provides file-system operations scoped to the Macros/ directory.
type MacrosService struct {
	root string
}

func NewMacrosService() *MacrosService {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "Macros")
	for i := 0; i < 5; i++ {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			break
		}
		wd = filepath.Dir(wd)
		root = filepath.Join(wd, "Macros")
	}
	return &MacrosService{root: root}
}

// GetRootPath returns the absolute path of the Macros directory.
func (s *MacrosService) GetRootPath() string {
	return s.root
}

// cleanPath resolves subpath relative to root and guards against traversal.
func (s *MacrosService) cleanPath(subpath string) string {
	clean := filepath.Clean(strings.TrimPrefix(subpath, s.root))
	clean = strings.TrimPrefix(clean, string(os.PathSeparator))
	return filepath.Join(s.root, clean)
}

// versionsDir returns the .versions directory for a given subpath.
func (s *MacrosService) versionsDir(subpath string) string {
	return filepath.Join(s.root, ".versions", subpath)
}

// metaPath is the metadata.json path.
func (s *MacrosService) metaPath() string {
	return filepath.Join(s.root, "metadata.json")
}

// fileSha1 computes the upper-case hex SHA1 of data.
func fileSha1(data []byte) string {
	h := sha1.Sum(data)
	return fmt.Sprintf("%X", h)
}

// windowsPath converts a forward-slash path to the Windows-backslash format used in metadata.json.
func windowsPath(subpath string) string {
	return strings.ReplaceAll(subpath, "/", "\\")
}

// ---------------------------------------------------------------------------
// Metadata helpers
// ---------------------------------------------------------------------------

func (s *MacrosService) readMeta() ([]FileMeta, error) {
	data, err := os.ReadFile(s.metaPath())
	if err != nil {
		return nil, err
	}
	var meta []FileMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *MacrosService) writeMeta(meta []FileMeta) error {
	data, err := json.MarshalIndent(meta, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.metaPath(), data, 0644)
}

// updateMeta finds the entry matching subpath and updates SHA1, Size, ModifiedDate.
// If no entry exists it's silently skipped (not an error).
func (s *MacrosService) updateMeta(subpath string, content []byte) error {
	meta, err := s.readMeta()
	if err != nil {
		return nil // metadata.json optional
	}
	wpath := windowsPath(subpath)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	sh := fileSha1(content)
	found := false
	for i := range meta {
		if strings.EqualFold(meta[i].FileName, wpath) {
			meta[i].SHA1 = sh
			meta[i].Size = len(content)
			meta[i].ModifiedDate = now
			found = true
			break
		}
	}
	if !found {
		return nil // not tracked in metadata
	}
	return s.writeMeta(meta)
}

// ---------------------------------------------------------------------------
// Version helpers
// ---------------------------------------------------------------------------

// backupFile copies the current file (if it exists) to .versions/<subpath>/<timestamp>.
func (s *MacrosService) backupFile(subpath string) error {
	src := s.cleanPath(subpath)
	info, err := os.Stat(src)
	if err != nil {
		return nil // file doesn't exist yet, nothing to back up
	}
	if info.IsDir() {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	verDir := s.versionsDir(subpath)
	if err := os.MkdirAll(verDir, 0755); err != nil {
		return err
	}
	ts := time.Now().UTC().Format("20060102_150405")
	verFile := filepath.Join(verDir, ts)
	return os.WriteFile(verFile, data, 0644)
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// ListDir returns entries inside the given subpath (or root if empty).
func (s *MacrosService) ListDir(subpath string) ([]FileEntry, error) {
	dir := s.root
	if subpath != "" {
		dir = s.cleanPath(subpath)
	}
	if !strings.HasPrefix(dir, s.root) {
		return nil, os.ErrPermission
	}
	// Hide .versions from directory listing
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		out = append(out, FileEntry{
			Name:  name,
			IsDir: e.IsDir(),
			Path:  filepath.ToSlash(filepath.Join(subpath, name)),
		})
	}
	return out, nil
}

// ReadFile returns the contents of the file at the given subpath.
func (s *MacrosService) ReadFile(subpath string) (string, error) {
	path := s.cleanPath(subpath)
	if !strings.HasPrefix(path, s.root) {
		return "", os.ErrPermission
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveFile writes content to the file, creates a version backup, and updates
// metadata.json (SHA1 / Size / ModifiedDate) when the file is tracked there.
func (s *MacrosService) SaveFile(subpath string, content string) error {
	path := s.cleanPath(subpath)
	if !strings.HasPrefix(path, s.root) {
		return os.ErrPermission
	}
	// 1. Create backup of current file
	if err := s.backupFile(subpath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	// 2. Ensure parent exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// 3. Write new content
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}
	// 4. Update metadata.json
	if err := s.updateMeta(subpath, []byte(content)); err != nil {
		return fmt.Errorf("metadata update failed: %w", err)
	}
	return nil
}

// GetVersions lists all saved versions for a file, newest first.
func (s *MacrosService) GetVersions(subpath string) ([]VersionInfo, error) {
	verDir := s.versionsDir(subpath)
	entries, err := os.ReadDir(verDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []VersionInfo{}, nil
		}
		return nil, err
	}
	out := make([]VersionInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		vp := filepath.Join(verDir, e.Name())
		info, err := os.Stat(vp)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(vp)
		if err != nil {
			continue
		}
		// Parse timestamp for display
		ts := e.Name()
		t, err := time.Parse("20060102_150405", ts)
		display := ts
		if err == nil {
			display = t.Local().Format("02 Jan 2006 15:04:05")
		}
		out = append(out, VersionInfo{
			VersionID:  ts,
			ModifiedAt: display,
			Size:       info.Size(),
			SHA1:       fileSha1(data),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].VersionID > out[j].VersionID // newest first
	})
	return out, nil
}

// GetVersionContent returns the content of a specific version.
func (s *MacrosService) GetVersionContent(subpath string, versionID string) (string, error) {
	vp := filepath.Join(s.versionsDir(subpath), versionID)
	if !strings.HasPrefix(vp, s.root) {
		return "", os.ErrPermission
	}
	data, err := os.ReadFile(vp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RestoreVersion replaces the current file with the given version and creates a
// backup of the current content first.
func (s *MacrosService) RestoreVersion(subpath string, versionID string) error {
	content, err := s.GetVersionContent(subpath, versionID)
	if err != nil {
		return err
	}
	return s.SaveFile(subpath, content)
}

// ---------------------------------------------------------------------------
// App Versioning
// ---------------------------------------------------------------------------

// AppVersionInfo holds the current app version and release date.
type AppVersionInfo struct {
	Version string `json:"version"`
	Date    string `json:"date"`
}

// VersionEntry is one entry in the version history.
type VersionEntry struct {
	Version   string `json:"version"`
	Date      string `json:"date"`
	Changelog string `json:"changelog"`
}

// VersionManifest is the full version.json structure.
type VersionManifest struct {
	Current AppVersionInfo `json:"current"`
	History []VersionEntry `json:"history"`
}

func (s *MacrosService) versionFilePath() string {
	return filepath.Join(filepath.Dir(s.root), "version.json")
}

func (s *MacrosService) readVersionManifest() (*VersionManifest, error) {
	data, err := os.ReadFile(s.versionFilePath())
	if err != nil {
		// Return a sensible default
		return &VersionManifest{
			Current: AppVersionInfo{Version: "0.0.1", Date: time.Now().UTC().Format(time.RFC3339)},
			History: []VersionEntry{},
		}, nil
	}
	var m VersionManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Current.Version == "" {
		m.Current.Version = "0.0.1"
	}
	return &m, nil
}

func (s *MacrosService) writeVersionManifest(m *VersionManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.versionFilePath(), data, 0644)
}

// bumpVersion increments the given semver string.
// bumpType: "major", "minor", or "patch" (default).
func bumpVersion(v string, bumpType string) string {
	parts := strings.SplitN(v, ".", 3)
	major := 0
	minor := 0
	patch := 0
	if len(parts) > 0 {
		fmt.Sscanf(parts[0], "%d", &major)
	}
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	if len(parts) > 2 {
		fmt.Sscanf(parts[2], "%d", &patch)
	}
	switch bumpType {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	default: // patch
		patch++
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}

// GetAppVersion returns the current app version info.
func (s *MacrosService) GetAppVersion() (*AppVersionInfo, error) {
	m, err := s.readVersionManifest()
	if err != nil {
		return nil, err
	}
	return &m.Current, nil
}

// GetVersionHistory returns all recorded versions.
func (s *MacrosService) GetVersionHistory() ([]VersionEntry, error) {
	m, err := s.readVersionManifest()
	if err != nil {
		return nil, err
	}
	return m.History, nil
}

// PublishVersion bumps the version and records a changelog entry.
// bumpType: "major", "minor", "patch".
func (s *MacrosService) PublishVersion(changelog string, bumpType string) (*AppVersionInfo, error) {
	m, err := s.readVersionManifest()
	if err != nil {
		return nil, err
	}
	newVer := bumpVersion(m.Current.Version, bumpType)
	now := time.Now().UTC()
	entry := VersionEntry{
		Version:   newVer,
		Date:      now.Format(time.RFC3339),
		Changelog: changelog,
	}
	m.Current = AppVersionInfo{Version: newVer, Date: now.Format(time.RFC3339)}
	m.History = append([]VersionEntry{entry}, m.History...)
	if err := s.writeVersionManifest(m); err != nil {
		return nil, err
	}
	return &m.Current, nil
}

// GetReleaseCommand returns the shell command to create a GitHub Release.
func (s *MacrosService) GetReleaseCommand() string {
	return "./deploy.sh"
}
