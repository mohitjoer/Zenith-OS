package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
)

//go:embed web
var webFS embed.FS

// Serve runs the dashboard and the agent bridge on 127.0.0.1.
func Serve(cfg Config) error {
	rt := RuntimeDir()
	if err := os.MkdirAll(rt, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(rt, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another " + binName + " daemon is already running")
	}
	pidfile := filepath.Join(rt, "daemon.pid")
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return err
	}
	defer os.Remove(pidfile)

	if err := EnsureVault(); err != nil {
		return err
	}
	go func() {
		if err := Reindex(); err != nil {
			fmt.Fprintln(os.Stderr, binName+": index:", err)
		}
	}()
	go func() {
		for range time.Tick(6 * time.Hour) {
			_ = Reindex()
		}
	}()
	// User-owned changes and the habits layer reindex separately from the
	// full map: a cheap fingerprint every 2 minutes, the rewrite only when
	// the live config or the habits sources (history, asks, runs, recipes)
	// move.
	go func() {
		lastCfg := userConfigFingerprint()
		lastHab := habitsFingerprint()
		for range time.Tick(2 * time.Minute) {
			if cur := userConfigFingerprint(); cur != lastCfg {
				lastCfg = cur
				_ = ReindexUser()
			}
			if cur := habitsFingerprint(); cur != lastHab {
				lastHab = cur
				_ = WriteHabits()
			}
		}
	}()

	hub := newChatHub()
	mux := http.NewServeMux()

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServerFS(sub))

	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, BuildStatus(cfg))
	})
	mux.HandleFunc("GET /api/vitals", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, SampleVitals())
	})
	mux.HandleFunc("GET /api/vault", func(w http.ResponseWriter, r *http.Request) {
		files, err := VaultTree()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"root": VaultDir(), "files": files})
	})
	mux.HandleFunc("GET /api/vault/file", func(w http.ResponseWriter, r *http.Request) {
		b, err := ReadVaultFile(r.URL.Query().Get("p"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("POST /api/index", func(w http.ResponseWriter, r *http.Request) {
		if err := Reindex(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/agents", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, DetectAgents())
	})
	mux.HandleFunc("POST /api/agents/wire", agentMutation(func(id string) error {
		if err := Wire(id); err != nil {
			return err
		}
		wireProwlSkills() // parity with the wire verb: pointer + skill + prowl
		return nil
	}))
	mux.HandleFunc("POST /api/agents/unwire", agentMutation(Unwire))
	mux.HandleFunc("GET /api/quick", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, quickInfo(LoadConfig()))
	})
	mux.HandleFunc("POST /api/quick", func(w http.ResponseWriter, r *http.Request) {
		if err := cmdBackend([]string{r.URL.Query().Get("provider")}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, quickInfo(LoadConfig()))
	})
	mux.HandleFunc("GET /api/manifest", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, BuildManifest(LoadConfig()))
	})

	mux.HandleFunc("GET /api/skills", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, SkillsReportNow())
	})
	mux.HandleFunc("GET /api/memory", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, MemoryReportNow())
	})
	mux.HandleFunc("GET /api/prowl", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, ProwlReportNow())
	})
	mux.HandleFunc("GET /api/prowl/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"hits": ProwlSearch(r.URL.Query().Get("q"))})
	})
	mux.HandleFunc("GET /api/about", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, AboutReportNow(cfg))
	})

	mux.HandleFunc("GET /ws/vitals", func(w http.ResponseWriter, r *http.Request) {
		ws, err := acceptWS(w, r)
		if err != nil {
			return
		}
		defer ws.CloseNow()
		serveVitalsWS(r.Context(), ws)
	})
	mux.HandleFunc("GET /ws/chat", func(w http.ResponseWriter, r *http.Request) {
		ws, err := acceptWS(w, r)
		if err != nil {
			return
		}
		defer ws.CloseNow()
		hub.handle(r.Context(), ws, r.URL.Query().Get("peek") == "1")
	})

	// The unix socket serves everything, plus the verbs only the CLI and the shell
	// call. The browser's loopback port never gets those: any web page can send a
	// request to 127.0.0.1, and /api/perm would let it answer an agent's approval.
	cliMux := http.NewServeMux()
	cliMux.Handle("/", mux)
	cliMux.HandleFunc("POST /api/ask", hub.handleAsk)
	cliMux.HandleFunc("POST /api/ask/cancel", hub.handleAskCancel)
	cliMux.HandleFunc("GET /api/ask/recent", hub.handleAskRecent)
	cliMux.HandleFunc("POST /api/term", hub.handleTerm)
	cliMux.HandleFunc("POST /api/term/cancel", hub.handleTermCancel)
	cliMux.HandleFunc("POST /api/term/ran", hub.handleTermRan)
	cliMux.HandleFunc("POST /api/perm", hub.handlePerm)

	// The flock above makes any socket file still lying here a leftover.
	_ = os.Remove(SocketPath())
	ul, err := net.Listen("unix", SocketPath())
	if err != nil {
		return err
	}
	defer os.Remove(SocketPath())
	if err := os.Chmod(SocketPath(), 0o600); err != nil {
		return err
	}
	unixSrv := &http.Server{Handler: cliMux, ReadHeaderTimeout: 5 * time.Second}
	srv := &http.Server{
		Addr:              net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port)),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 2)
	go func() { errCh <- unixSrv.Serve(ul) }()
	go func() { errCh <- srv.ListenAndServe() }()
	fmt.Printf(binName+": serving %s and dashboard on http://%s\n", SocketPath(), srv.Addr)

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = unixSrv.Shutdown(shutCtx)
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func agentMutation(f func(string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.ID == "" {
			http.Error(w, "missing agent id", http.StatusBadRequest)
			return
		}
		if err := f(body.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, a := range DetectAgents() {
			if a.ID == body.ID {
				writeJSON(w, a)
				return
			}
		}
		http.Error(w, "unknown agent", http.StatusBadRequest)
	}
}

// acceptWS upgrades only when the Origin is this machine's own dashboard.
func acceptWS(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return nil, errors.New("bad origin")
		}
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"127.0.0.1:*", "localhost:*"},
	})
	if err == nil {
		// User messages carry base64 image blocks; the 32 KiB default read
		// limit rejects all but the smallest, so raise it past a downscaled image.
		conn.SetReadLimit(8 << 20)
	}
	return conn, err
}

func cmdServe(ifEnabled bool) error {
	cfg := LoadConfig()
	if ifEnabled && !cfg.Enabled {
		return nil
	}
	return Serve(cfg)
}

// systemd helpers: the daemon prefers running as a systemd user unit so it starts
// with every login (and with the machine itself once lingering is on),
// survives Hyprland restarts, and restarts on crash. Everything degrades to
// the plain detached spawn when systemd is not around.

func systemctlUser(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// unitKnown reports whether the user unit file is installed anywhere systemd
// looks (package: /usr/lib/systemd/user, dev deploy: ~/.config/systemd/user).
func unitKnown() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "--user", "list-unit-files", unitName, "--no-legend").Output()
	return err == nil && strings.Contains(string(out), unitName)
}

func haveSystemd() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return os.Getenv("XDG_RUNTIME_DIR") != ""
}

func cmdEnable(atBoot bool) error {
	cfg := LoadConfig()
	cfg.Enabled = true
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	if haveSystemd() && unitKnown() {
		// An old detached daemon would hold the flock and wedge the unit.
		stopSpawnedDaemon()
		if err := systemctlUser("enable", "--now", unitName); err != nil {
			return err
		}
		if atBoot {
			// Lingering starts the user manager (and this unit) at BOOT,
			// before login. Needs polkit auth or root once.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			lc := exec.CommandContext(ctx, "loginctl", "enable-linger", currentUser())
			lc.Stdout, lc.Stderr = os.Stdout, os.Stderr
			if err := lc.Run(); err != nil {
				fmt.Fprintln(os.Stderr, binName+": enable-linger failed (dashboard will start at login instead of boot):", err)
			} else {
				fmt.Println(displayName + " enabled (starts at boot via lingering)")
				return nil
			}
		}
		fmt.Println(displayName + " enabled (systemd user unit, starts at login)")
		return nil
	}
	// No systemd: fall back to a detached spawn for this session only.
	if !pingDaemon() {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.Command(self, "serve")
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return err
		}
		_ = cmd.Process.Release()
	}
	fmt.Println(displayName + " enabled")
	return nil
}

func cmdDisable() error {
	cfg := LoadConfig()
	cfg.Enabled = false
	if err := SaveConfig(cfg); err != nil {
		return err
	}
	if haveSystemd() && unitKnown() {
		_ = systemctlUser("disable", "--now", unitName)
	}
	stopSpawnedDaemon()
	fmt.Println(displayName + " disabled")
	return nil
}

// stopSpawnedDaemon SIGTERMs a daemon started by the pre-systemd spawn path.
func stopSpawnedDaemon() {
	b, err := os.ReadFile(filepath.Join(RuntimeDir(), "daemon.pid"))
	if err != nil {
		return
	}
	if pid, perr := strconv.Atoi(strings.TrimSpace(string(b))); perr == nil {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return strconv.Itoa(os.Getuid())
}

func pingDaemon() bool {
	resp, err := daemonClient(500 * time.Millisecond).Get(daemonURL("/api/ping"))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// daemonClient returns an HTTP client that dials the daemon's unix socket. The
// host in daemonURL is never resolved.
func daemonClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", SocketPath())
			},
		},
	}
}

func daemonURL(path string) string { return "http://" + binName + path }
