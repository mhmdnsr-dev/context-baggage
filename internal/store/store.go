package store

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	Home string
}

func New(home string) Store {
	return Store{Home: home}
}

func (s Store) Init() error {
	for _, dir := range []string{
		s.Home,
		filepath.Join(s.Home, "inventory", "claude"),
		filepath.Join(s.Home, "inventory", "codex"),
		filepath.Join(s.Home, "workspaces"),
		filepath.Join(s.Home, "sync"),
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if _, err := os.Stat(s.ConfigPath()); errors.Is(err, os.ErrNotExist) {
		if err := s.WriteConfig(Config{Version: "0.1"}); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) ConfigPath() string { return filepath.Join(s.Home, "config.yaml") }
func (s Store) DevicePath() string { return filepath.Join(s.Home, "device.yaml") }
func (s Store) SyncPath() string   { return filepath.Join(s.Home, "sync", "state.yaml") }
func (s Store) WorkspaceDir(id string) string {
	return filepath.Join(s.Home, "workspaces", id)
}
func (s Store) WorkspacePath(id string) string {
	return filepath.Join(s.WorkspaceDir(id), "workspace.yaml")
}
func (s Store) TaskDir(workspaceID, taskID string) string {
	return filepath.Join(s.WorkspaceDir(workspaceID), "tasks", taskID)
}
func (s Store) TaskPath(workspaceID, taskID string) string {
	return filepath.Join(s.TaskDir(workspaceID, taskID), "task.yaml")
}
func (s Store) CheckpointsPath(workspaceID, taskID string) string {
	return filepath.Join(s.TaskDir(workspaceID, taskID), "checkpoints.jsonl")
}
func (s Store) HandoffPath(workspaceID, taskID string) string {
	return filepath.Join(s.TaskDir(workspaceID, taskID), "handoff.md")
}
func (s Store) ActiveTaskPath(workspaceID string) string {
	return filepath.Join(s.WorkspaceDir(workspaceID), "active-task")
}
func (s Store) InventoryPath(agent string) string {
	return filepath.Join(s.Home, "inventory", agent, "inventory.yaml")
}

func Now() string { return time.Now().UTC().Format(time.RFC3339) }

func NewID(prefix string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b[:]), nil
}

func StableID(prefix, value string) string {
	// Stable IDs are derived from normalized logical identity, not local paths.
	// That is what lets the same repository be recognized across machines.
	sum := sha256.Sum256([]byte(value))
	return prefix + "_" + hex.EncodeToString(sum[:])[:16]
}

func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Canonical files should not be left half-written if the process exits or
	// the OS interrupts the write. Write beside the target, flush, then rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// Best-effort cleanup. Once rename succeeds this path no longer exists;
		// on earlier failures the operation has already returned its real error.
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(data); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return fmt.Errorf("write temporary file: %w; close temporary file: %v", err, closeErr)
		}
		return err
	}
	if err := tmp.Sync(); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return fmt.Errorf("sync temporary file: %w; close temporary file: %v", err, closeErr)
		}
		return err
	}
	// Close must succeed before the temporary file is renamed. Some filesystem
	// write failures can surface only when the file is closed.
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (s Store) WriteConfig(c Config) error {
	return AtomicWrite(s.ConfigPath(), []byte("version: "+c.Version+"\n"), 0o600)
}

func (s Store) ReadDevice() (Device, error) {
	kv, err := readKV(s.DevicePath())
	if err != nil {
		return Device{}, err
	}
	return Device{ID: kv["id"], Name: kv["name"], OS: kv["os"], Arch: kv["arch"], CreatedAt: kv["createdAt"]}, nil
}

func (s Store) WriteDevice(d Device) error {
	data := fmt.Sprintf("id: %s\nname: %s\nos: %s\narch: %s\ncreatedAt: %s\n", d.ID, d.Name, d.OS, d.Arch, d.CreatedAt)
	return AtomicWrite(s.DevicePath(), []byte(data), 0o600)
}

func (s Store) ReadWorkspace(id string) (Workspace, error) {
	lines, err := readLines(s.WorkspacePath(id))
	if err != nil {
		return Workspace{}, err
	}
	return parseWorkspace(lines)
}

func (s Store) WriteWorkspace(w Workspace) error {
	sort.Strings(w.LocalPaths)
	lines := []string{
		fmt.Sprintf("id: %s", w.ID),
		fmt.Sprintf("name: %s", w.Name),
		"identity:",
		fmt.Sprintf("  type: %s", w.Identity.Type),
		fmt.Sprintf("  value: %s", w.Identity.Value),
		"localPaths:",
	}
	for _, p := range w.LocalPaths {
		lines = append(lines, fmt.Sprintf("  - %s", p))
	}
	lines = append(lines,
		fmt.Sprintf("sync: %t", w.Sync),
		fmt.Sprintf("createdAt: %s", w.CreatedAt),
		fmt.Sprintf("updatedAt: %s", w.UpdatedAt),
	)
	return AtomicWrite(s.WorkspacePath(w.ID), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// Portable returns the explicit portable projection of a workspace.
func (w Workspace) Portable() PortableWorkspace {
	return PortableWorkspace{ID: w.ID, Name: w.Name, Identity: w.Identity, Sync: w.Sync, CreatedAt: w.CreatedAt}
}

// ApplyPortable overwrites the portable-authoritative fields from p while
// keeping machine-local fields from the receiver unchanged.
func (w Workspace) ApplyPortable(p PortableWorkspace) Workspace {
	w.ID = p.ID
	w.Name = p.Name
	w.Identity = p.Identity
	w.Sync = p.Sync
	w.CreatedAt = p.CreatedAt
	return w
}

// WritePortableWorkspace writes a projected workspace.yaml into dir. It emits
// only portable-authoritative fields and never machine-local ones.
func WritePortableWorkspace(dir string, p PortableWorkspace) error {
	lines := []string{
		fmt.Sprintf("id: %s", p.ID),
		fmt.Sprintf("name: %s", p.Name),
		"identity:",
		fmt.Sprintf("  type: %s", p.Identity.Type),
		fmt.Sprintf("  value: %s", p.Identity.Value),
		fmt.Sprintf("sync: %t", p.Sync),
		fmt.Sprintf("createdAt: %s", p.CreatedAt),
	}
	return AtomicWrite(filepath.Join(dir, "workspace.yaml"), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// ReadPortableWorkspace reads a projected workspace.yaml from dir.
func ReadPortableWorkspace(dir string) (PortableWorkspace, error) {
	lines, err := readLines(filepath.Join(dir, "workspace.yaml"))
	if err != nil {
		return PortableWorkspace{}, err
	}
	w, err := parseWorkspace(lines)
	if err != nil {
		return PortableWorkspace{}, err
	}
	return w.Portable(), nil
}

func (s Store) ListWorkspaces() ([]Workspace, error) {
	entries, err := os.ReadDir(filepath.Join(s.Home, "workspaces"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Workspace
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		w, err := s.ReadWorkspace(e.Name())
		if err == nil {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s Store) WriteTask(t Task) error {
	data := fmt.Sprintf("id: %s\nname: %s\nworkspaceId: %s\nstatus: %s\ncreatedAt: %s\nupdatedAt: %s\n", t.ID, t.Name, t.WorkspaceID, t.Status, t.CreatedAt, t.UpdatedAt)
	if err := AtomicWrite(s.TaskPath(t.WorkspaceID, t.ID), []byte(data), 0o600); err != nil {
		return err
	}
	cp := s.CheckpointsPath(t.WorkspaceID, t.ID)
	if _, err := os.Stat(cp); errors.Is(err, os.ErrNotExist) {
		if err := AtomicWrite(cp, nil, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) ReadTask(workspaceID, taskID string) (Task, error) {
	kv, err := readKV(s.TaskPath(workspaceID, taskID))
	if err != nil {
		return Task{}, err
	}
	return Task{ID: kv["id"], Name: kv["name"], WorkspaceID: kv["workspaceId"], Status: kv["status"], CreatedAt: kv["createdAt"], UpdatedAt: kv["updatedAt"]}, nil
}

func (s Store) ListTasks(workspaceID string) ([]Task, error) {
	dir := filepath.Join(s.WorkspaceDir(workspaceID), "tasks")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Task
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := s.ReadTask(workspaceID, e.Name())
		if err == nil {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s Store) SetActiveTask(workspaceID, taskID string) error {
	return AtomicWrite(s.ActiveTaskPath(workspaceID), []byte(taskID+"\n"), 0o600)
}

func (s Store) ActiveTask(workspaceID string) (string, error) {
	data, err := os.ReadFile(s.ActiveTaskPath(workspaceID))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s Store) AppendCheckpoint(workspaceID, taskID string, record any) error {
	f, err := os.OpenFile(s.CheckpointsPath(workspaceID, taskID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(record); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return fmt.Errorf("write checkpoint: %w; close checkpoint file: %v", err, closeErr)
		}
		return err
	}
	return f.Close()
}

func (s Store) ReadSync() (SyncState, error) {
	kv, err := readKV(s.SyncPath())
	if err != nil {
		return SyncState{}, err
	}
	return SyncState{Folder: kv["folder"], LastPush: kv["lastPush"], LastPull: kv["lastPull"], LastPushHash: kv["lastPushHash"], LastPullHash: kv["lastPullHash"], BaseHash: kv["baseHash"]}, nil
}

func (s Store) WriteSync(st SyncState) error {
	data := fmt.Sprintf("folder: %s\nlastPush: %s\nlastPull: %s\nlastPushHash: %s\nlastPullHash: %s\nbaseHash: %s\n", st.Folder, st.LastPush, st.LastPull, st.LastPushHash, st.LastPullHash, st.BaseHash)
	return AtomicWrite(s.SyncPath(), []byte(data), 0o600)
}

func (s Store) WriteInventory(agent string, inv AgentInventory) error {
	lines := []string{
		fmt.Sprintf("name: %s", inv.Name),
		fmt.Sprintf("detected: %t", inv.Detected),
		fmt.Sprintf("updatedAt: %s", inv.UpdatedAt),
		"configPaths:",
	}
	for _, p := range inv.ConfigPaths {
		lines = append(lines, fmt.Sprintf("  - %s", p))
	}
	lines = append(lines, "metadata:")
	keys := make([]string, 0, len(inv.Metadata))
	for k := range inv.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("  %s: %s", k, inv.Metadata[k]))
	}
	lines = append(lines, "warnings:")
	for _, w := range inv.Warnings {
		lines = append(lines, fmt.Sprintf("  - %s", w))
	}
	return AtomicWrite(s.InventoryPath(agent), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func HashDir(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, err := h.Write([]byte(filepath.ToSlash(rel))); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			// Read-only close is best-effort; hashing has already consumed the file.
			_ = f.Close()
		}()
		_, err = io.Copy(h, f)
		return err
	})
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readKV(path string) (map[string]string, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") || strings.HasSuffix(strings.TrimSpace(line), ":") || strings.HasPrefix(strings.TrimSpace(line), "- ") {
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 {
			out[parts[0]] = strings.TrimSpace(parts[1])
		}
	}
	return out, nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Read-only close is best-effort; scanner errors are returned separately.
		_ = f.Close()
	}()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out, sc.Err()
}

func parseWorkspace(lines []string) (Workspace, error) {
	var w Workspace
	section := ""
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(raw, " ") && strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}
		if section == "localPaths" && strings.HasPrefix(line, "- ") {
			w.LocalPaths = append(w.LocalPaths, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], strings.TrimSpace(parts[1])
		switch {
		case section == "identity" && key == "type":
			w.Identity.Type = val
		case section == "identity" && key == "value":
			w.Identity.Value = val
		case key == "id":
			w.ID = val
		case key == "name":
			w.Name = val
		case key == "sync":
			w.Sync = val == "true"
		case key == "createdAt":
			w.CreatedAt = val
		case key == "updatedAt":
			w.UpdatedAt = val
		}
	}
	if w.ID == "" {
		return Workspace{}, errors.New("workspace id missing")
	}
	return w, nil
}
