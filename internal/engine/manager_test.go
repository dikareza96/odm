package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// heldManager returns a started manager whose downloads hang on a server that
// never answers until the test ends, so slot assignment can be observed
// without transfers finishing underneath the assertions.
func heldManager(t *testing.T, concurrent int) (*Manager, string) {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	m := NewManager(Settings{Dir: t.TempDir(), Concurrent: concurrent}, "")
	ctx, cancel := context.WithCancel(context.Background())
	m.Begin(ctx)
	t.Cleanup(func() {
		cancel()
		close(release)
		srv.Close()
	})
	return m, srv.URL + "/f.bin"
}

func activeCount(m *Manager) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

func addHeld(m *Manager, url string, start bool) Info {
	return m.Create(AddOptions{URL: url, Start: start})
}

func TestManualDownloadsUseAllConcurrentSlots(t *testing.T) {
	m, url := heldManager(t, 3)
	for i := 0; i < 4; i++ {
		addHeld(m, url, true)
	}
	if got := activeCount(m); got != 3 {
		t.Fatalf("active = %d, want 3 (Concurrent cap, not one per queue)", got)
	}
}

func TestStartedQueueRunsOneAtATime(t *testing.T) {
	m, url := heldManager(t, 3)
	for i := 0; i < 3; i++ {
		addHeld(m, url, false)
	}
	m.StartQueue(DefaultQueue)
	if got := activeCount(m); got != 1 {
		t.Fatalf("active = %d, want 1 for a started queue", got)
	}
}

// allDays is a schedule's weekday set with every day ticked.
func allDays() []bool { return []bool{true, true, true, true, true, true, true} }

func TestStartedQueueHonoursItsParallelCount(t *testing.T) {
	m, url := heldManager(t, 5)
	m.settings.Schedules = map[string]QueueSchedule{DefaultQueue: {Parallel: 2, Days: allDays()}}
	for i := 0; i < 4; i++ {
		addHeld(m, url, false)
	}
	m.StartQueue(DefaultQueue)
	if got := activeCount(m); got != 2 {
		t.Fatalf("active = %d, want the queue's 2 files at a time", got)
	}
}

func TestScheduleStartsAndStopsPerQueue(t *testing.T) {
	// No Begin: nothing is dispatched, so only the scheduler's decisions are
	// observed (queue started or not, item queued or paused).
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.AddQueue("Night")
	at := time.Date(2026, 10, 13, 23, 0, 5, 0, time.Local) // a Tuesday
	m.settings.Schedules["Night"] = QueueSchedule{
		StartEnabled: true, StartAt: "23:00", Days: allDays(),
		StopEnabled: true, StopAt: "07:30", Parallel: 1,
	}
	night := m.Create(AddOptions{URL: "http://127.0.0.1:1/n.bin", Queue: "Night"})
	manual := m.Create(AddOptions{URL: "http://127.0.0.1:1/m.bin", Start: true}) // Main, started by hand
	running := func(q string) bool { return m.QueueRunning(q) }

	m.checkSchedule(at)
	if !running("Night") || running(DefaultQueue) {
		t.Fatal("23:00 should start Night only")
	}
	m.StopQueue("Night") // user stops it; the same minute must not restart it
	m.checkSchedule(at.Add(20 * time.Second))
	if running("Night") {
		t.Fatal("start re-fired within the same minute")
	}

	// Next morning's stop applies to the running queue only.
	m.StartQueue("Night")
	morning := time.Date(2026, 10, 14, 7, 30, 0, 0, time.Local)
	m.checkSchedule(morning)
	if running("Night") || m.get(night.ID).status() != StatusPaused {
		t.Fatal("07:30 did not stop Night")
	}
	if m.get(manual.ID).status() != StatusQueued {
		t.Fatal("stop time paused a download that the schedule did not start")
	}

	// A disabled weekday does not start it.
	wed := at.AddDate(0, 0, 1)
	sc := m.settings.Schedules["Night"]
	sc.Days[int(wed.Weekday())] = false
	m.settings.Schedules["Night"] = sc
	m.checkSchedule(wed)
	if running("Night") {
		t.Fatal("started on a disabled weekday")
	}
}

func TestScheduleOnceDate(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.settings.Schedules[DefaultQueue] = QueueSchedule{StartEnabled: true, StartAt: "09:00", OnceDate: "2026-10-20", Days: allDays()}
	m.Create(AddOptions{URL: "http://127.0.0.1:1/a.bin"})
	m.checkSchedule(time.Date(2026, 10, 19, 9, 0, 0, 0, time.Local))
	if m.QueueRunning(DefaultQueue) {
		t.Fatal("started before the once date")
	}
	m.checkSchedule(time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local))
	if !m.QueueRunning(DefaultQueue) {
		t.Fatal("did not start on the once date")
	}
}

func TestWhenDoneActions(t *testing.T) {
	newM := func(sc QueueSchedule) (*Manager, *[]Action) {
		m := NewManager(Settings{Dir: t.TempDir()}, "")
		sc.Days = allDays()
		m.settings.Schedules[DefaultQueue] = sc
		var mu sync.Mutex
		fired := &[]Action{}
		m.SetActionHandler(func(a Action) { mu.Lock(); *fired = append(*fired, a); mu.Unlock() })
		return m, fired
	}
	put := func(m *Manager, st Status) {
		d := New(newID(), "http://x/f", m.settings.Dir, 1, nil, 0, m.client)
		d.Status = st
		m.downloads[d.ID] = d
		m.order = append(m.order, d.ID)
	}

	// A manual download finishing with nothing started: no action.
	m, fired := newM(QueueSchedule{ShutdownWhenDone: true})
	put(m, StatusComplete)
	m.whenDone()
	if len(*fired) != 0 {
		t.Fatalf("fired %v after a manual download", *fired)
	}

	// A started queue with a paused item has not finished: no action.
	m, fired = newM(QueueSchedule{ShutdownWhenDone: true})
	put(m, StatusComplete)
	put(m, StatusPaused)
	m.queueRun[DefaultQueue] = true
	m.settleQueuesLocked()
	m.whenDone()
	if len(*fired) != 0 {
		t.Fatalf("fired %v while the queue still had a paused item", *fired)
	}

	// Finished queue: open-file at once, shutdown once, exit superseded.
	m, fired = newM(QueueSchedule{OpenWhenDone: "/tmp/done.txt", ExitWhenDone: true, ShutdownWhenDone: true})
	put(m, StatusComplete)
	put(m, StatusError)
	m.queueRun[DefaultQueue] = true
	m.settleQueuesLocked()
	m.whenDone()
	m.whenDone()
	if len(*fired) != 2 || (*fired)[0].Kind != ActionOpen || (*fired)[0].Path != "/tmp/done.txt" || (*fired)[1].Kind != ActionShutdown {
		t.Fatalf("fired %+v, want [open, shutdown] once", *fired)
	}

	// Shutdown waits while another started queue is still running.
	m, fired = newM(QueueSchedule{ShutdownWhenDone: true})
	m.AddQueue("Night")
	put(m, StatusComplete)
	m.queueRun[DefaultQueue] = true
	m.queueRun["Night"] = true
	m.drained = []string{DefaultQueue}
	delete(m.queueRun, DefaultQueue)
	m.whenDone()
	if len(*fired) != 0 {
		t.Fatalf("shutdown fired while Night was still running: %+v", *fired)
	}
	delete(m.queueRun, "Night")
	m.whenDone()
	if len(*fired) != 1 || (*fired)[0].Kind != ActionShutdown {
		t.Fatalf("held shutdown not fired once everything stopped: %+v", *fired)
	}
}

func TestRetryInStartedQueue(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.settings.Schedules[DefaultQueue] = QueueSchedule{RetriesEnabled: true, Retries: 2, Days: allDays(), Parallel: 1}
	d := New("r1", "http://x/f", m.settings.Dir, 1, nil, 0, m.client)
	d.Status = StatusError
	m.downloads[d.ID] = d
	m.order = append(m.order, d.ID)

	if m.retryInQueue(d) {
		t.Fatal("retried although the queue was not started")
	}
	m.queueRun[DefaultQueue] = true
	for i := 0; i < 2; i++ {
		d.Status = StatusError
		if !m.retryInQueue(d) || d.status() != StatusQueued {
			t.Fatalf("attempt %d not re-queued", i+1)
		}
	}
	d.Status = StatusError
	if m.retryInQueue(d) {
		t.Fatal("retried beyond the configured count")
	}
}

func TestDownloadLimitPausesAndBlocks(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	m.settings.Limits = DownloadLimits{Enabled: true, MB: 1, Hours: 1}
	var got []Action
	m.SetActionHandler(func(a Action) { got = append(got, a) })
	d := New("l1", "http://x/f", m.settings.Dir, 1, nil, 0, m.client)
	d.Chunks = []*Chunk{{ID: 0, Start: 0, End: 4 << 20}}
	d.Status = StatusQueued
	m.downloads[d.ID] = d
	m.order = append(m.order, d.ID)

	t0 := time.Date(2026, 10, 13, 10, 0, 0, 0, time.Local)
	m.trackLimit(t0) // baseline
	d.Chunks[0].Done = 600 << 10
	m.trackLimit(t0.Add(time.Second))
	if m.limitHit {
		t.Fatal("hit below the cap")
	}
	d.Chunks[0].Done = 1200 << 10
	m.trackLimit(t0.Add(2 * time.Second))
	if !m.limitHit || d.status() != StatusPaused || len(got) != 1 || got[0].Kind != ActionLimit {
		t.Fatalf("cap crossed: hit=%v status=%s actions=%v", m.limitHit, d.status(), got)
	}
	m.trackLimit(t0.Add(2*time.Hour + 3*time.Second)) // window has passed
	if m.limitHit {
		t.Fatal("limit not cleared after the window")
	}
}

func waitStatus(t *testing.T, d *Download, want Status) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for d.status() != want {
		if time.Now().After(deadline) {
			t.Fatalf("status %s, want %s", d.status(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestPauseBeforeStartIsHonoured(t *testing.T) {
	var hits int64
	srv := rangeServer(t, make([]byte, 1<<20), &hits)
	defer srv.Close()
	d := New("p1", srv.URL+"/f.bin", t.TempDir(), 2, nil, 0, srv.Client())

	// Dispatched but Start not yet running: no cancel func exists.
	d.Pause()
	d.Start(context.Background())
	if d.status() != StatusPaused {
		t.Fatalf("status %s, want paused", d.status())
	}
	if hits != 0 {
		t.Fatalf("%d requests made for a download paused before it started", hits)
	}

	// The request is consumed: the next run must transfer normally.
	d.Start(context.Background())
	if d.status() != StatusComplete {
		t.Fatalf("second run: status %s (%s), want complete", d.status(), d.Err)
	}
}

func TestPauseDuringProbeIsPausedNotError(t *testing.T) {
	_, url := heldManager(t, 1) // only its never-answering server is used
	d := New("p2", url, t.TempDir(), 2, nil, 0, http.DefaultClient)
	done := make(chan struct{})
	go func() { d.Start(context.Background()); close(done) }()
	waitStatus(t, d, StatusDownloading)
	d.Pause()
	<-done
	if d.status() != StatusPaused {
		t.Fatalf("status %s (%s), want paused", d.status(), d.Err)
	}
}

func TestMoveInQueueReordersWithinQueue(t *testing.T) {
	m := NewManager(Settings{Dir: t.TempDir()}, "")
	a := m.Create(AddOptions{URL: "http://h/a.zip"})
	b := m.Create(AddOptions{URL: "http://h/b.zip"})
	other := m.Create(AddOptions{URL: "http://h/c.zip", Queue: "Night"})
	order := func(id string) int { i, _ := m.Get(id); return i.Order }

	if !m.MoveInQueue(b.ID, -1) {
		t.Fatal("move up refused")
	}
	if order(b.ID) >= order(a.ID) {
		t.Fatalf("b (%d) should now precede a (%d)", order(b.ID), order(a.ID))
	}
	if m.MoveInQueue(b.ID, -1) {
		t.Fatal("moved past the top of the queue")
	}
	if m.MoveInQueue(other.ID, +1) {
		t.Fatal("single-item queue moved into another queue")
	}
}
