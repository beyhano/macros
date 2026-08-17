package main

import (
	"context"
	"crypto/sha1"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed Macros
var macrosEmbed embed.FS

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
	root := resolveMacrosRoot()
	log.Printf("Macros kök dizini: %s", root)
	return &MacrosService{root: root}
}

// Macro library sync configuration.
const (
	macroRepoURL = "https://github.com/beyhano/ClassicAssist-Macros.git"
	macroRepoDir = "ClassicAssist-Macros"
)

// resolveMacrosRoot decides where the macro library lives:
//  1. An existing synced checkout of ClassicAssist-Macros → its Macros/ subdir (auto pull).
//  2. A fresh clone of ClassicAssist-Macros into cwd (or home) → its Macros/ subdir.
//  3. A legacy Macros/ directory found in cwd or home.
//  4. Fallback: extract the embedded library into cwd.
func resolveMacrosRoot() string {
	wd, _ := os.Getwd()
	home := os.Getenv("HOME")
	var bases []string
	if wd != "" {
		bases = append(bases, wd)
	}
	if home != "" && home != wd {
		bases = append(bases, home)
	}
	return pickLibraryRoot(bases, isGitRepo, cloneMacroRepo, extractTo)
}

// extractTo extracts the embedded library into target; reports success.
func extractTo(target string) bool {
	if err := extractEmbedded(macrosEmbed, "Macros", target); err != nil {
		log.Printf("Uyarı: Macros/ çıkartılamadı: %v", err)
		return false
	}
	return true
}

// pickLibraryRoot implements the library resolution decision (injectable for
// tests): checkouts win, a fresh clone runs when no checkout exists, a legacy
// Macros/ dir is used when cloning is impossible, and extraction is the final
// fallback. All paths are absolute and returned as-is.
func pickLibraryRoot(bases []string, gitRepo, cloneFn func(string) bool, extractFn func(string) bool) string {
	for _, base := range bases {
		repo := filepath.Join(base, macroRepoDir)
		if gitRepo(repo) {
			if lib := filepath.Join(repo, "Macros"); isDir(lib) {
				pullMacroRepo(repo)
				return lib
			}
		}
	}
	for _, base := range bases {
		repo := filepath.Join(base, macroRepoDir)
		if !isDir(repo) && cloneFn(repo) {
			if lib := filepath.Join(repo, "Macros"); isDir(lib) {
				return lib
			}
		}
	}
	for _, base := range bases {
		candidate := filepath.Join(base, "Macros")
		if isDir(candidate) {
			return candidate
		}
	}
	if len(bases) > 0 {
		root := filepath.Join(bases[0], "Macros")
		log.Println("Macros/ bulunamadı, embedded dosyalar çıkartılıyor...")
		if extractFn(root) {
			log.Println("Macros/ başarıyla oluşturuldu.")
			return root
		}
	}
	return filepath.Join(".", "Macros")
}

// cloneMacroRepo clones ClassicAssist-Macros (shallow) into target.
// Returns true on success; existing checkouts are left untouched.
func cloneMacroRepo(target string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	log.Printf("ClassicAssist-Macros klonlanıyor: %s", target)
	_, err := git.PlainCloneContext(ctx, target, false, &git.CloneOptions{
		URL:      macroRepoURL,
		Depth:    1,
		Progress: nil,
	})
	if err != nil {
		log.Printf("Uyarı: Klon başarısız: %v", err)
		return false
	}
	return true
}

// pullMacroRepo fast-forwards the checkout. Failures (offline, local changes)
// are logged and ignored — the existing library keeps working.
func pullMacroRepo(repo string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	log.Println("ClassicAssist-Macros güncelleniyor (pull)...")
		repository, err := git.PlainOpen(repo)
	if err != nil {
		log.Printf("Uyarı: repo açılamadı: %v", err)
		return
	}
	wt, err := repository.Worktree()
	if err != nil {
		log.Printf("Uyarı: worktree açılamadı: %v", err)
		return
	}
	if err := wt.PullContext(ctx, &git.PullOptions{Force: false, SingleBranch: true}); err != nil {
		if err == git.NoErrAlreadyUpToDate {
			log.Println("Kütüphane güncel.")
			return
		}
		log.Printf("Uyarı: Pull başarısız (çevrimdışı veya yerel değişiklik?): %v", err)
		return
	}
	log.Println("Kütüphane güncellendi.")
}

// isGitRepo reports whether dir contains a .git entry (file or directory).
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// isDir reports whether path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// extractEmbedded copies an embedded fs directory to a target path on disk.
// Skips files/dirs starting with "." (e.g. .versions).
func extractEmbedded(efs embed.FS, srcPath string, dstPath string) error {
	return fs.WalkDir(efs, srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden entries
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(srcPath, path)
		target := filepath.Join(dstPath, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := efs.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
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
// Helpers for CreateMacro
// ---------------------------------------------------------------------------

// CreateFile creates a new .py file under Macros/ with the given name.
// parentPath is an optional subdirectory (e.g. "Crafting" or "Skills/Magery").
// Returns the subpath (forward-slash) of the created file.
func (s *MacrosService) CreateFile(name string, parentPath string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("file name is required")
	}
	ext := filepath.Ext(name)
	if ext == "" {
		name += ".py"
	} else if ext != ".py" {
		name += ".py"
	}
	parent := strings.Trim(strings.ReplaceAll(parentPath, "\\", "/"), "/")
	subpath := name
	if parent != "" {
		subpath = parent + "/" + name
	}
	subpath = filepath.ToSlash(subpath)
	dest := s.cleanPath(subpath)
	if !strings.HasPrefix(dest, s.root) {
		return "", os.ErrPermission
	}
	// Dedup if exists
	base := strings.TrimSuffix(name, ".py")
	for i := 1; ; i++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		fname := fmt.Sprintf("%s_%d.py", base, i)
		subpath = fname
		if parent != "" {
			subpath = parent + "/" + fname
		}
		subpath = filepath.ToSlash(subpath)
		dest = s.cleanPath(subpath)
	}
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	content := fmt.Sprintf("# %s\n\n", name)
	if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
		return "", err
	}
	return subpath, nil
}

// MoveFile moves a file from sourcePath to targetDir (both relative to Macros root).
// Returns the new subpath of the moved file.
func (s *MacrosService) MoveFile(sourcePath string, targetDir string) (string, error) {
	if sourcePath == "" {
		return "", fmt.Errorf("source path is required")
	}
	src := s.cleanPath(sourcePath)
	if !strings.HasPrefix(src, s.root) {
		return "", os.ErrPermission
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("cannot move a directory")
	}
	// Build destination path
	dir := strings.Trim(strings.ReplaceAll(targetDir, "\\", "/"), "/")
	fname := filepath.Base(src)
	newPath := dir + "/" + fname
	if dir == "" {
		newPath = fname
	}
	newPath = filepath.ToSlash(newPath)
	dest := s.cleanPath(newPath)
	if !strings.HasPrefix(dest, s.root) {
		return "", os.ErrPermission
	}
	// Dedup if destination exists
	base := strings.TrimSuffix(fname, ".py")
	for i := 1; ; i++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		fname2 := fmt.Sprintf("%s_%d.py", base, i)
		newPath = dir + "/" + fname2
		if dir == "" {
			newPath = fname2
		}
		newPath = filepath.ToSlash(newPath)
		dest = s.cleanPath(newPath)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", err
	}
	if err := os.Rename(src, dest); err != nil {
		return "", err
	}
	return newPath, nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// ListDirs returns all subdirectory paths recursively under the given subpath (or root if empty).
func (s *MacrosService) ListDirs(subpath string) ([]string, error) {
	dir := s.root
	if subpath != "" {
		dir = s.cleanPath(subpath)
	}
	if !strings.HasPrefix(dir, s.root) {
		return nil, os.ErrPermission
	}
	var dirs []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		rel, _ := filepath.Rel(s.root, path)
		if rel == "." {
			return nil // skip root
		}
		dirs = append(dirs, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dirs, nil
}

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
	// version.json sits next to the executable (works for dev and installed
	// builds regardless of where the macro library root lives).
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "version.json")
	}
	return filepath.Join(filepath.Dir(s.root), "version.json")
}

func (s *MacrosService) readVersionManifest() (*VersionManifest, error) {
	data, err := os.ReadFile(s.versionFilePath())
	// Fall back to the manifest embedded into the binary when there is no
	// version.json on disk (e.g. installer-only install or bare binary run).
	if err != nil {
		if embedded, eerr := embeddedVersionManifest(); eerr == nil {
			return embedded, nil
		}
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

// embeddedVersionManifest returns the version manifest embedded into the
// binary at build time (module-root version.json).
func embeddedVersionManifest() (*VersionManifest, error) {
	var m VersionManifest
	if err := json.Unmarshal(versionJSON, &m); err != nil {
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

// CheckForUpdates asks the Wails updater for a newer release. Returns true
// when an update is available and its window has been opened; false when the
// app is already up to date. Errors (e.g. offline, no release) are returned,
// never fatal.
func (s *MacrosService) CheckForUpdates() (bool, error) {
	app := application.Get()
	if app == nil {
		return false, nil
	}
	ctx := context.Background()
	rel, err := app.Updater.Check(ctx)
	if err != nil {
		return false, err
	}
	if rel == nil {
		return false, nil
	}
	go app.Updater.CheckAndInstall(ctx)
	return true, nil
}
