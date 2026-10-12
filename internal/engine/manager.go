package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"odm/internal/osutil"
)

// Manager owns every download, the concurrency limit, settings and persistence.
type Manager struct {
	mu        sync.Mutex
	downloads map[string]*Download
	order     []string
	running   map[string]bool
	active    int
	client    *http.Client
	store     *Store
	settings  Settings
	setPath   string
	limiter   *Limiter
	onChange  func()
	ctx       context.Context
}

// NewManager builds a manager from persisted settings.
func NewManager(settings Settings, settingsPath string, client *http.Client) *Manager {
	settings.normalize(settings.Dir)
	if client == nil {
		client = &http.Client{Timeout: 0}
	}
	return &Manager{
		downloads: map[string]*Download{},
		running:   map[string]bool{},
		client:    client,
		settings:  settings,
		setPath:   settingsPath,
		limiter:   NewLimiter(settings.SpeedLimit),
	}
}

// SetStore attaches persistence.
func (m *Manager) SetStore(s *Store) { m.store = s }

// SetOnChange registers a callback fired whenever any download changes.
func (m *Manager) SetOnChange(fn func()) { m.onChange = fn }

// Settings returns a copy of the current settings.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// UpdateSettings validates, persists and applies new settings live.
func (m *Manager) UpdateSettings(next Settings) Settings {
	m.mu.Lock()
	fallback := m.settings.Dir
	next.normalize(fallback)
	m.settings = next
	path := m.setPath
	m.mu.Unlock()

	m.limiter.SetRate(next.SpeedLimit)
	if path != "" {
		_ = SaveSettings(path, next)
	}
	m.fire()
	return next
}

// Restore loads persisted downloads into the manager.
func (m *Manager) Restore(states []State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, st := range states {
		d := Restore(st, m.client)
		d.Categories = m.settings.Categories
		d.SetLimiter(m.limiter)
		d.SetOnUpdate(m.changed)
		m.downloads[d.ID] = d
		m.order = append(m.order, d.ID)
	}
}

// Begin binds the run context and starts any queued downloads. Call it before
// Add so the pump has a context to launch work with.
func (m *Manager) Begin(ctx context.Context) {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
	m.pump()
}

// Start begins the background sampler/pump loop; it blocks until ctx ends.
func (m *Manager) Start(ctx context.Context) {
	m.Begin(ctx)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sample()
		}
	}
}

func (m *Manager) sample() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	any := false
	for _, id := range ids {
		if d := m.get(id); d != nil {
			d.Sample()
			any = true
		}
	}
	if any {
		m.fire()
	}
}

// AddOptions describes a new download.
type AddOptions struct {
	URL          string
	Dir          string
	Connections  int
	Headers      map[string]string
	SpeedLimit   int64
	Start        bool
	OpenWhenDone bool
	Category     string
	FileName     string
}

// Create queues a download from opts and returns its projection.
func (m *Manager) Create(o AddOptions) Info {
	m.mu.Lock()
	st := m.settings
	m.mu.Unlock()

	dir := o.Dir
	if dir == "" {
		dir = st.Dir
	}
	conns := o.Connections
	if conns <= 0 {
		conns = st.Connections
	}
	d := New(newID(), o.URL, dir, conns, o.Headers, o.SpeedLimit, m.client)
	d.Categories = st.Categories
	d.CategoryName = o.Category
	d.FileName = o.FileName
	d.OpenWhenDone = o.OpenWhenDone
	d.SetLimiter(m.limiter)
	if o.SpeedLimit > 0 {
		d.SetSpeedLimit(o.SpeedLimit)
	}
	if !o.Start {
		d.mu.Lock()
		d.Status = StatusPaused
		d.mu.Unlock()
	}
	d.SetOnUpdate(m.changed)

	m.mu.Lock()
	m.downloads[d.ID] = d
	m.order = append(m.order, d.ID)
	m.mu.Unlock()
	m.fire()
	m.pump()
	return d.Info()
}

// Probe inspects a URL without creating a download (used by the Add dialog).
func (m *Manager) Probe(ctx context.Context, rawURL string, headers map[string]string) (*Probe, error) {
	return Inspect(ctx, m.client, rawURL, headers)
}

// SetSpeedLimit applies a per-download cap in bytes/sec.
func (m *Manager) SetSpeedLimit(id string, bytesPerSecond int64) bool {
	d := m.get(id)
	if d == nil {
		return false
	}
	d.SetSpeedLimit(bytesPerSecond)
	m.changed(d)
	return true
}

// SetOpenWhenDone toggles opening the file on completion.
func (m *Manager) SetOpenWhenDone(id string, v bool) bool {
	d := m.get(id)
	if d == nil {
		return false
	}
	d.SetOpenWhenDone(v)
	m.changed(d)
	return true
}

// Add queues a new download and returns its projection.
func (m *Manager) Add(rawURL, dir string, conns int, headers map[string]string, speedLimit int64) Info {
	return m.Create(AddOptions{URL: rawURL, Dir: dir, Connections: conns, Headers: headers, SpeedLimit: speedLimit, Start: true})
}

// List returns every download, oldest first.
func (m *Manager) List() []Info {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	out := make([]Info, 0, len(ids))
	for _, id := range ids {
		if d := m.get(id); d != nil {
			out = append(out, d.Info())
		}
	}
	return out
}

// Get returns one download by id.
func (m *Manager) Get(id string) (Info, bool) {
	if d := m.get(id); d != nil {
		return d.Info(), true
	}
	return Info{}, false
}

// Pause stops a download.
func (m *Manager) Pause(id string) bool {
	d := m.get(id)
	if d == nil {
		return false
	}
	m.mu.Lock()
	running := m.running[id]
	m.mu.Unlock()
	if running {
		d.Pause()
	} else {
		d.mu.Lock()
		if d.Status == StatusQueued {
			d.Status = StatusPaused
		}
		d.mu.Unlock()
		m.changed(d)
	}
	return true
}

// Resume restarts a paused or failed download.
func (m *Manager) Resume(id string) bool {
	if d := m.get(id); d != nil {
		m.resume(d)
	}
	return true
}

func (m *Manager) resume(d *Download) {
	d.mu.Lock()
	if d.Status != StatusComplete {
		d.Status = StatusQueued
		d.Err = ""
	}
	d.mu.Unlock()
	m.pump()
}

// PauseAll pauses every active or queued download.
func (m *Manager) PauseAll() {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	for _, id := range ids {
		m.Pause(id)
	}
}

// ResumeAll restarts every paused or failed download.
func (m *Manager) ResumeAll() {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	for _, id := range ids {
		m.resume(m.get(id))
	}
}

// ClearCompleted removes finished entries (their files are kept).
func (m *Manager) ClearCompleted() int {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	n := 0
	for _, id := range ids {
		if d := m.get(id); d != nil && d.status() == StatusComplete {
			if m.Remove(id) {
				n++
			}
		}
	}
	return n
}

// Remove stops the download and drops its state. When deleteFile is true the
// partial or finished file is deleted from disk too.
func (m *Manager) Remove(id string) bool { return m.RemoveWithFile(id, false) }

// RemoveWithFile stops the download, deletes its state and optionally its file.
func (m *Manager) RemoveWithFile(id string, deleteFile bool) bool {
	d := m.get(id)
	if d == nil {
		return false
	}
	d.Pause()
	d.mu.Lock()
	dir, name := d.Dir, d.FileName
	f := d.file
	d.mu.Unlock()
	if f != nil {
		_ = f.Close()
	}

	m.mu.Lock()
	delete(m.downloads, id)
	delete(m.running, id)
	for i, v := range m.order {
		if v == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.mu.Unlock()

	if name != "" {
		_ = os.Remove(filepath.Join(dir, name+".odmpart"))
		if deleteFile {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	if m.store != nil {
		_ = m.store.Delete(id)
	}
	m.fire()
	m.pump()
	return true
}

// Path returns the on-disk path of a download for open/reveal actions.
func (m *Manager) Path(id string) (string, bool) {
	d := m.get(id)
	if d == nil {
		return "", false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.FileName == "" {
		return "", false
	}
	p := filepath.Join(d.Dir, d.FileName)
	if d.Status != StatusComplete {
		p += ".odmpart"
	}
	return p, true
}

func (m *Manager) get(id string) *Download {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.downloads[id]
}

func (m *Manager) pump() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx == nil {
		return
	}
	for _, id := range m.order {
		if m.active >= m.settings.Concurrent {
			return
		}
		d := m.downloads[id]
		if d == nil || m.running[id] {
			continue
		}
		if d.status() == StatusQueued {
			m.running[id] = true
			m.active++
			go m.run(d)
		}
	}
}

func (m *Manager) run(d *Download) {
	d.Start(m.ctx)
	m.mu.Lock()
	delete(m.running, d.ID)
	if m.active > 0 {
		m.active--
	}
	m.mu.Unlock()

	if d.status() == StatusComplete {
		d.mu.Lock()
		open, dir, name := d.OpenWhenDone, d.Dir, d.FileName
		d.mu.Unlock()
		if open && name != "" {
			_ = osutil.Open(filepath.Join(dir, name))
		}
	}
	m.changed(d)
	m.pump()
}

func (m *Manager) changed(d *Download) {
	if m.store != nil {
		_ = m.store.Save(d.State())
	}
	m.fire()
}

func (m *Manager) fire() {
	if m.onChange != nil {
		m.onChange()
	}
}

func (d *Download) status() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.Status
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405")
	}
	return hex.EncodeToString(b)
}
