// Command zenith-userd is the unprivileged user-level daemon for Zenith OS.
//
// It runs as the user under ~/.config/service/zenith-userd and acts as:
//  1. State Authority: watches ~/.config/zenith/shell.json and applies themes/fonts
//     to GTK3/4, Qt6, GSettings, and Hyprland.
//  2. IPC Bridge: streams telemetry to Quickshell and forwards privileged calls
//     to /run/zenith.sock (zenith-sysd).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	sysSocketPath = "/run/zenith.sock"
	version       = "0.1-beta"
)

type ShellConfig struct {
	Theme        string `json:"theme"`
	IconTheme    string `json:"icon_theme"`
	FontName     string `json:"font_name"`
	FontSize     int    `json:"font_size"`
	DarkMode     bool   `json:"dark_mode"`
	BorderRadius int    `json:"border_radius"`
	GapsIn       int    `json:"gaps_in"`
	GapsOut      int    `json:"gaps_out"`
	PowerMode    string `json:"power_mode"`
}

type UserIPCRequest struct {
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type UserIPCResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type UserDaemon struct {
	configDir  string
	configPath string
	userSocket string
	config     ShellConfig
	mu         sync.RWMutex
	clients    map[net.Conn]bool
	clientsMu  sync.Mutex
}

func main() {
	userSocket := getUserSocketPath()
	d := &UserDaemon{
		userSocket: userSocket,
		clients:    make(map[net.Conn]bool),
	}

	d.initPaths()
	d.loadConfig()
	d.applyAllSettings()

	go d.watchConfigFile()
	go d.telemetryLoop()

	d.runServer()
}

func getUserSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return filepath.Join(runtimeDir, "zenith-user.sock")
}

func (d *UserDaemon) initPaths() {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	d.configDir = filepath.Join(configHome, "zenith")
	d.configPath = filepath.Join(d.configDir, "shell.json")
	_ = os.MkdirAll(d.configDir, 0755)
}

func (d *UserDaemon) loadConfig() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Default clean monochrome settings
	d.config = ShellConfig{
		Theme:        "Adwaita-dark",
		IconTheme:    "Papirus-Dark",
		FontName:     "Inter",
		FontSize:     10,
		DarkMode:     true,
		BorderRadius: 8,
		GapsIn:       4,
		GapsOut:      8,
		PowerMode:    "balanced",
	}

	if data, err := os.ReadFile(d.configPath); err == nil {
		_ = json.Unmarshal(data, &d.config)
	} else {
		// Save default configuration
		if out, err := json.MarshalIndent(d.config, "", "  "); err == nil {
			_ = os.WriteFile(d.configPath, out, 0644)
		}
	}
}

func (d *UserDaemon) watchConfigFile() {
	var lastMod time.Time
	for {
		time.Sleep(1 * time.Second)
		fi, err := os.Stat(d.configPath)
		if err != nil {
			continue
		}
		if fi.ModTime().After(lastMod) {
			lastMod = fi.ModTime()
			d.loadConfig()
			d.applyAllSettings()
			d.broadcastEvent("config_updated", d.config)
		}
	}
}

func (d *UserDaemon) applyAllSettings() {
	d.mu.RLock()
	cfg := d.config
	d.mu.RUnlock()

	// 1. GTK 3 & GTK 4 settings.ini
	home := os.Getenv("HOME")
	gtkThemes := fmt.Sprintf("[Settings]\ngtk-theme-name=%s\ngtk-icon-theme-name=%s\ngtk-font-name=%s %d\ngtk-application-prefer-dark-theme=%d\n",
		cfg.Theme, cfg.IconTheme, cfg.FontName, cfg.FontSize, boolToInt(cfg.DarkMode))

	for _, dir := range []string{filepath.Join(home, ".config", "gtk-3.0"), filepath.Join(home, ".config", "gtk-4.0")} {
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "settings.ini"), []byte(gtkThemes), 0644)
	}

	// 2. GSettings (if running)
	if _, err := exec.LookPath("gsettings"); err == nil {
		_ = exec.Command("gsettings", "set", "org.gnome.desktop.interface", "gtk-theme", cfg.Theme).Run()
		_ = exec.Command("gsettings", "set", "org.gnome.desktop.interface", "icon-theme", cfg.IconTheme).Run()
		_ = exec.Command("gsettings", "set", "org.gnome.desktop.interface", "font-name", fmt.Sprintf("%s %d", cfg.FontName, cfg.FontSize)).Run()
		if cfg.DarkMode {
			_ = exec.Command("gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "prefer-dark").Run()
		} else {
			_ = exec.Command("gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "default").Run()
		}
	}

	// 3. Hyprland dynamic rules
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("hyprctl"); err == nil {
			_ = exec.Command("hyprctl", "keyword", "general:gaps_in", fmt.Sprintf("%d", cfg.GapsIn)).Run()
			_ = exec.Command("hyprctl", "keyword", "general:gaps_out", fmt.Sprintf("%d", cfg.GapsOut)).Run()
			_ = exec.Command("hyprctl", "keyword", "decoration:rounding", fmt.Sprintf("%d", cfg.BorderRadius)).Run()
		}
	}

	// 4. Forward power mode to zenith-sysd if needed
	go func() {
		_, _ = forwardToSysd(map[string]interface{}{
			"action":  "set_power_profile",
			"profile": cfg.PowerMode,
		})
	}()
}

func (d *UserDaemon) runServer() {
	_ = os.Remove(d.userSocket)
	listener, err := net.Listen("unix", d.userSocket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "zenith-userd: failed to listen on %s: %v\n", d.userSocket, err)
		os.Exit(1)
	}
	defer os.Remove(d.userSocket)
	defer listener.Close()

	_ = os.Chmod(d.userSocket, 0600)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		_ = listener.Close()
		_ = os.Remove(d.userSocket)
		os.Exit(0)
	}()

	fmt.Printf("zenith-userd: listening on %s\n", d.userSocket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		d.clientsMu.Lock()
		d.clients[conn] = true
		d.clientsMu.Unlock()
		go d.handleClient(conn)
	}
}

func (d *UserDaemon) handleClient(conn net.Conn) {
	defer func() {
		d.clientsMu.Lock()
		delete(d.clients, conn)
		d.clientsMu.Unlock()
		conn.Close()
	}()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		var req UserIPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendUserResponse(conn, UserIPCResponse{Status: "error", Message: "invalid JSON"})
			continue
		}

		res := d.handleRequest(req)
		sendUserResponse(conn, res)
	}
}

func (d *UserDaemon) handleRequest(req UserIPCRequest) UserIPCResponse {
	switch req.Action {
	case "get_config":
		d.mu.RLock()
		cfg := d.config
		d.mu.RUnlock()
		return UserIPCResponse{Status: "ok", Data: cfg}

	case "set_theme":
		var update struct {
			Theme     string `json:"theme,omitempty"`
			DarkMode  *bool  `json:"dark_mode,omitempty"`
			PowerMode string `json:"power_mode,omitempty"`
		}
		if err := json.Unmarshal(req.Payload, &update); err == nil {
			d.mu.Lock()
			if update.Theme != "" {
				d.config.Theme = update.Theme
			}
			if update.DarkMode != nil {
				d.config.DarkMode = *update.DarkMode
			}
			if update.PowerMode != "" {
				d.config.PowerMode = update.PowerMode
			}
			d.saveConfigLocked()
			d.mu.Unlock()
			d.applyAllSettings()
			return UserIPCResponse{Status: "ok", Message: "theme updated"}
		}
		return UserIPCResponse{Status: "error", Message: "bad payload"}

	case "sys_hardware":
		resp, err := forwardToSysd(map[string]interface{}{"action": "hardware_scan"})
		if err != nil {
			return UserIPCResponse{Status: "error", Message: err.Error()}
		}
		return UserIPCResponse{Status: "ok", Data: resp}

	case "sys_power":
		var p struct {
			Profile string `json:"profile"`
		}
		_ = json.Unmarshal(req.Payload, &p)
		resp, err := forwardToSysd(map[string]interface{}{
			"action":  "set_power_profile",
			"profile": p.Profile,
		})
		if err != nil {
			return UserIPCResponse{Status: "error", Message: err.Error()}
		}
		return UserIPCResponse{Status: "ok", Data: resp}

	default:
		return UserIPCResponse{Status: "error", Message: "unknown action: " + req.Action}
	}
}

func (d *UserDaemon) saveConfigLocked() {
	if out, err := json.MarshalIndent(d.config, "", "  "); err == nil {
		_ = os.WriteFile(d.configPath, out, 0644)
	}
}

func (d *UserDaemon) telemetryLoop() {
	for {
		time.Sleep(3 * time.Second)
		data := map[string]interface{}{
			"time":    time.Now().Format("15:04"),
			"battery": readBattery(),
		}
		d.broadcastEvent("telemetry", data)
	}
}

func (d *UserDaemon) broadcastEvent(event string, payload interface{}) {
	msg, err := json.Marshal(map[string]interface{}{
		"event":   event,
		"payload": payload,
	})
	if err != nil {
		return
	}
	msg = append(msg, '\n')

	d.clientsMu.Lock()
	defer d.clientsMu.Unlock()
	for client := range d.clients {
		_, _ = client.Write(msg)
	}
}

func forwardToSysd(req interface{}) (interface{}, error) {
	conn, err := net.Dial("unix", sysSocketPath)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to zenith-sysd: %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}

	var res map[string]interface{}
	if err := json.Unmarshal(line, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func sendUserResponse(conn net.Conn, res UserIPCResponse) {
	data, err := json.Marshal(res)
	if err == nil {
		_, _ = conn.Write(append(data, '\n'))
	}
}

func readBattery() string {
	if capBytes, err := os.ReadFile("/sys/class/power_supply/BAT0/capacity"); err == nil {
		return strings.TrimSpace(string(capBytes)) + "%"
	}
	return "AC"
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
