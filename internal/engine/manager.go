package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
func NewManager(settings Settings, settingsPath string) *Manager {
	settings.normalize(settings.Dir)
	return &Manager{
		downloads: map[string]*Download{},
		running:   map[string]bool{},
		client:    buildClient(settings),
		settings:  settings,
		setPath:   settingsPath,
		limiter:   NewLimiter(settings.SpeedLimit),
	}
}

// buildClient creates an HTTP client honouring the configured proxy.
func buildClient(s Settings) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     30 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	if s.Proxy.Mode == "http" && strings.TrimSpace(s.Proxy.Host) != "" {
		host := net.JoinHostPort(strings.TrimSpace(s.Proxy.Host), strconv.Itoa(s.Proxy.Port))
		pu := &url.URL{Scheme: "http", Host: host}
		if s.Proxy.User != "" {
			pu.User = url.UserPassword(s.Proxy.User, s.Proxy.Pass)
		}
		transport.Proxy = http.ProxyURL(pu)
	}
	return &http.Client{Transport: transport}
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
	proxyChanged := m.settings.Proxy != next.Proxy
	next.normalize(fallback)
	m.settings = next
	path := m.setPath
	if proxyChanged {
		m.client = buildClient(next)
	}
	m.mu.Unlock()

	m.limiter.SetRate(next.SpeedLimit)
	if path != "" {
		_ = SaveSettings(path, next)
	}
	m.fire()
	return next
}

// credentials returns the stored Basic credentials for a host, if any.
func (m *Manager) credentials(host string) (string, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, l := range m.settings.Logins {
		if strings.EqualFold(strings.TrimSpace(l.Host), strings.TrimSpace(host)) && l.User != "" {
			return l.User, l.Pass, true
		}
	}
	return "", "", false
}

/* ---------- queues ---------- */

// Queues returns the known queue names.
func (m *Manager) Queues() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.settings.Queues))
	copy(out, m.settings.Queues)
	return out
}

// AddQueue registers a new queue name.
func (m *Manager) AddQueue(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	m.mu.Lock()
	for _, q := range m.settings.Queues {
		if strings.EqualFold(q, name) {
			m.mu.Unlock()
			return false
		}
	}
	m.settings.Queues = append(m.settings.Queues, name)
	next := m.settings
	path := m.setPath
	m.mu.Unlock()
	if path != "" {
		_ = SaveSettings(path, next)
	}
	m.fire()
	return true
}

// MoveToQueue reassigns a download to another queue.
func (m *Manager) MoveToQueue(id, queue string) bool {
	d := m.get(id)
	if d == nil {
		return false
	}
	d.mu.Lock()
	d.Queue = queue
	d.mu.Unlock()
	m.changed(d)
	return true
}

// StartQueue resumes every finished-able download belonging to a queue.
func (m *Manager) StartQueue(name string) {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	for _, id := range ids {
		d := m.get(id)
		if d == nil {
			continue
		}
		d.mu.Lock()
		match := queueMatches(d.Queue, name)
		d.mu.Unlock()
		if match {
			m.resume(d)
		}
	}
}

// StopQueue pauses the active downloads of a queue.
func (m *Manager) StopQueue(name string) {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	for _, id := range ids {
		d := m.get(id)
		if d == nil {
			continue
		}
		d.mu.Lock()
		match := queueMatches(d.Queue, name)
		d.mu.Unlock()
		if match {
			m.Pause(id)
		}
	}
}

func queueMatches(downloadQueue, want string) bool {
	if want == "" || want == "*" {
		return true
	}
	q := downloadQueue
	if q == "" {
		q = DefaultQueue
	}
	return strings.EqualFold(q, want)
}

/* ---------- export / import ---------- */

// Export serialises the download list as JSON.
func (m *Manager) Export() ([]byte, error) {
	m.mu.Lock()
	ids := append([]string(nil), m.order...)
	m.mu.Unlock()
	states := make([]State, 0, len(ids))
	for _, id := range ids {
		if d := m.get(id); d != nil {
			states = append(states, d.State())
		}
	}
	return json.MarshalIndent(states, "", "  ")
}

// Import merges a previously exported list, returning how many were added.
func (m *Manager) Import(data []byte) (int, error) {
	var states []State
	if err := json.Unmarshal(data, &states); err != nil {
		return 0, err
	}
	added := 0
	for _, st := range states {
		if st.URL == "" {
			continue
		}
		if st.ID == "" {
			st.ID = newID()
		}
		m.mu.Lock()
		_, exists := m.downloads[st.ID]
		stt := m.settings
		m.mu.Unlock()
		if exists {
			continue
		}
		st.Status = StatusPaused
		if st.Queue == "" {
			st.Queue = DefaultQueue
		}
		d := Restore(st, m.client)
		d.Categories = stt.Categories
		d.SetLimiter(m.limiter)
		d.SetAuthLookup(m.credentials)
		d.SetOnUpdate(m.changed)

		m.mu.Lock()
		m.downloads[d.ID] = d
		m.order = append(m.order, d.ID)
		m.mu.Unlock()
		m.changed(d)
		added++
	}
	m.pump()
	return added, nil
}

/* ---------- lifecycle ---------- */

// Restore loads persisted downloads into the manager.
func (m *Manager) Restore(states []State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, st := range states {
		d := Restore(st, m.client)
		d.Categories = m.settings.Categories
		d.SetLimiter(m.limiter)
		d.SetAuthLookup(m.credentials)
		d.SetOnUpdate(m.changed)
		m.downloads[d.ID] = d
		m.order = append(m.order, d.ID)
	}
}

// Begin binds the run context and starts any queued downloads.
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
	Queue        string
	Description  string
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
	queue := strings.TrimSpace(o.Queue)
	if queue == "" {
		queue = DefaultQueue
	}
	d := New(newID(), o.URL, dir, conns, o.Headers, o.SpeedLimit, m.client)
	d.Categories = st.Categories
	d.CategoryName = o.Category
	d.FileName = o.FileName
	d.OpenWhenDone = o.OpenWhenDone
	d.Queue = queue
	d.Description = o.Description
	d.SetLimiter(m.limiter)
	d.SetAuthLookup(m.credentials)
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

// Probe inspects a URL without creating a download.
func (m *Manager) Probe(ctx context.Context, rawURL string, headers map[string]string) (*Probe, error) {
	return Inspect(ctx, m.client, strings.TrimSpace(rawURL), headers)
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

// Remove stops the download and drops its state.
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
