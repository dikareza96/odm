// Command odm is Ochi Download Manager: a segmented, resumable download engine
// with a local web UI shown in a WebView2/Edge app window on Windows.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"odm/internal/clipmon"
	"odm/internal/engine"
	"odm/internal/server"
	"odm/internal/ui"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8721", "listen address")
	dir := flag.String("dir", "", "download folder (blank = saved setting / ~/Downloads)")
	conns := flag.Int("conns", 0, "connections per download (0 = saved setting)")
	files := flag.Int("files", 0, "simultaneous downloads (0 = saved setting)")
	add := flag.String("add", "", "add a URL, wait for it to finish, then exit")
	nativeHost := flag.Bool("native-messaging", false, "run as a browser native-messaging host")
	open := flag.Bool("open", true, "open the UI window on start")
	noUI := flag.Bool("no-ui", false, "never open a UI window")
	flag.Parse()

	if *nativeHost {
		runNativeHost(*addr)
		return
	}

	baseDir := configBaseDir()
	if f, err := os.OpenFile(filepath.Join(baseDir, "odm.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		log.SetOutput(f)
		defer f.Close()
	}
	settingsPath := filepath.Join(baseDir, "settings.json")
	fallback := defaultDownloadDir()
	settings := engine.LoadSettings(settingsPath, fallback)
	if *dir != "" {
		settings.Dir = *dir
	}
	if *conns > 0 {
		settings.Connections = *conns
	}
	if *files > 0 {
		settings.Concurrent = *files
	}

	store, err := engine.NewStore(filepath.Join(baseDir, "state"))
	if err != nil {
		log.Println("cannot create state dir:", err)
		os.Exit(1)
	}

	mgr := engine.NewManager(settings, settingsPath)
	mgr.SetStore(store)
	if states, err := store.Load(); err == nil {
		mgr.Restore(states)
	}

	srv := server.New(mgr)
	mgr.SetOnChange(srv.Broadcast)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mgr.Begin(ctx)
	go mgr.Start(ctx)

	if settings.Clipboard {
		go clipmon.Watch(ctx, func(u string) {
			mgr.Add(u, "", 0, nil, 0)
		})
	}

	httpSrv := &http.Server{Addr: *addr, Handler: srv.Handler()}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("server:", err)
		}
	}()

	url := "http://" + *addr

	if *add != "" {
		info := mgr.Add(*add, settings.Dir, settings.Connections, nil, 0)
		fmt.Printf("added %s (%s)\n", info.FileName, info.ID)
		waitFor(ctx, mgr, info.ID)
		shutdown(httpSrv)
		return
	}

	fmt.Printf("Ochi Download Manager listening on %s\n", url)
	fmt.Printf("state: %s\ndownloads: %s\n", baseDir, settings.Dir)

	// On Windows the UI is a native Win32 window (no browser). The loopback API
	// stays up as an internal control channel for the browser relay.
	if runtime.GOOS == "windows" && *open && !*noUI {
		if err := ui.Run(mgr); err != nil {
			log.Println("ui:", err)
		}
		shutdown(httpSrv)
		return
	}

	if *open && !*noUI {
		openUI(url)
	}
	<-ctx.Done()
	shutdown(httpSrv)
}

func waitFor(ctx context.Context, mgr *engine.Manager, id string) {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	var last int64 = -1
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			info, ok := mgr.Get(id)
			if !ok {
				fmt.Println("download disappeared")
				return
			}
			if info.Done != last {
				last = info.Done
				fmt.Printf("\r  %s  %5.1f%%  %d/%d", info.Status, info.Progress, info.Done, info.Total)
			}
			switch info.Status {
			case engine.StatusComplete:
				fmt.Printf("\ndone: %s\n", info.FilePath)
				return
			case engine.StatusError:
				fmt.Printf("\nfailed: %s\n", info.Error)
				return
			}
		}
	}
}

func shutdown(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads")
}

func configBaseDir() string {
	cfg, err := os.UserConfigDir()
	if err != nil {
		cfg = os.TempDir()
	}
	return filepath.Join(cfg, "odm")
}

// openUI shows the control panel. On Windows it launches Microsoft Edge in
// application mode, which renders the page with the installed WebView2 runtime.
func openUI(url string) {
	switch runtime.GOOS {
	case "windows":
		if edge := findEdge(); edge != "" {
			profile := filepath.Join(os.TempDir(), "odm-webview")
			if err := exec.Command(edge, "--app="+url, "--user-data-dir="+profile, "--no-first-run").Start(); err == nil {
				return
			}
		}
		_ = exec.Command("cmd", "/c", "start", "", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

func findEdge() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Microsoft", "Edge", "Application", "msedge.exe"),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
