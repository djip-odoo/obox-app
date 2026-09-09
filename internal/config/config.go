package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrNoAvailablePort = errors.New("no available port in range")

const AppName = "EposProxy"

const (
	PortRangeStart = 4545
	PortRangeEnd   = 4555
)

type KioskConfig struct {
	Enabled bool `json:"enabled"`
}

type AppConfig struct {
	Port               int         `json:"port"`
	LANPrinters        []string    `json:"lan_printers,omitempty"`
	WebViewURL         string      `json:"webview_url,omitempty"`
	WebViewPIN         string      `json:"webview_pin,omitempty"`
	WebViewEnabled     bool        `json:"webview_enabled"`
	WebViewExitCorners []string    `json:"webview_exit_corners,omitempty"`
	NetworkPrinting    bool        `json:"network_printing"`
	Kiosk              KioskConfig `json:"kiosk,omitempty"`
}

func defaults() AppConfig {
	return AppConfig{
		Port:               0,
		NetworkPrinting:    false,
		WebViewPIN:         "0000",
		WebViewExitCorners: []string{"top-right"},
		Kiosk: KioskConfig{
			Enabled: false,
		},
	}
}

type Manager struct {
	mu   sync.RWMutex
	path string
	Data AppConfig
}

func isSystemDir(dir string) bool {
	clean := strings.ToLower(filepath.Clean(dir))
	base := filepath.Base(clean)
	return base == "system32" || base == "syswow64" || base == "windows"
}

func NewManager() (*Manager, error) {
	// 1. Check if config.json exists in the executable's directory
	if execPath, err := os.Executable(); err == nil {
		execDirConfig := filepath.Join(filepath.Dir(execPath), "config.json")
		if _, err := os.Stat(execDirConfig); err == nil {
			return &Manager{
				path: execDirConfig,
				Data: defaults(),
			}, nil
		}
	}

	// 2. Check %ProgramData%\EposProxy\config.json (standard on Windows for services/all users)
	if programData := os.Getenv("ProgramData"); programData != "" {
		pdConfig := filepath.Join(programData, AppName, "config.json")
		if _, err := os.Stat(pdConfig); err == nil {
			return &Manager{
				path: pdConfig,
				Data: defaults(),
			}, nil
		}
	}

	// 3. Check current working directory ONLY if it's not a Windows system directory (e.g. C:\Windows\System32)
	if cwd, err := os.Getwd(); err == nil && !isSystemDir(cwd) {
		cwdConfig := filepath.Join(cwd, "config.json")
		if _, err := os.Stat(cwdConfig); err == nil {
			return &Manager{
				path: cwdConfig,
				Data: defaults(),
			}, nil
		}
	}

	// 4. Try user config directory
	base, err := os.UserConfigDir()
	if err == nil {
		dir := filepath.Join(base, AppName)
		if err := os.MkdirAll(dir, 0755); err == nil {
			return &Manager{
				path: filepath.Join(dir, "config.json"),
				Data: defaults(),
			}, nil
		}
	}

	// 5. Fallback for services / LocalSystem when UserConfigDir fails:
	// Use %ProgramData%\EposProxy or executable directory
	if programData := os.Getenv("ProgramData"); programData != "" {
		dir := filepath.Join(programData, AppName)
		if err := os.MkdirAll(dir, 0755); err == nil {
			return &Manager{
				path: filepath.Join(dir, "config.json"),
				Data: defaults(),
			}, nil
		}
	}

	if execPath, err := os.Executable(); err == nil {
		return &Manager{
			path: filepath.Join(filepath.Dir(execPath), "config.json"),
			Data: defaults(),
		}, nil
	}

	return nil, fmt.Errorf("cannot locate or create any valid config directory")
}

func (cm *Manager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("config read error: %w", err)
	}

	if err := json.Unmarshal(data, &cm.Data); err != nil {
		return fmt.Errorf("config parse error: %w", err)
	}
	return nil
}

func (cm *Manager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.saveLocked()
}

func (cm *Manager) saveLocked() error {
	data, err := json.MarshalIndent(cm.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("config marshal error: %w", err)
	}
	if err := os.WriteFile(cm.path, data, 0644); err != nil {
		return fmt.Errorf("config write error: %w", err)
	}
	return nil
}

func (cm *Manager) Path() string { return cm.path }

func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func findAvailablePort(start, end int) (int, error) {
	for p := start; p <= end; p++ {
		if isPortAvailable(p) {
			return p, nil
		}
	}

	return 0, fmt.Errorf("no available port found in range %d-%d: %w", start, end, ErrNoAvailablePort)
}

func (cm *Manager) ResolvePort() (int, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.Data.Port > 0 && isPortAvailable(cm.Data.Port) {
		return cm.Data.Port, nil
	}

	port, err := findAvailablePort(PortRangeStart, PortRangeEnd)
	if err != nil {
		return 0, err
	}

	cm.Data.Port = port
	if err := cm.saveLocked(); err != nil {
		log.Printf("[config] warning: could not save: %v\n", err)
	}
	return port, nil
}

func (cm *Manager) GetPort() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.Port
}

func (cm *Manager) SetNetworkPrintingEnabled(enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.NetworkPrinting = enabled
	return cm.saveLocked()
}

func (cm *Manager) IsNetworkPrintingEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.NetworkPrinting
}

func (cm *Manager) AddLanEposPrinter(ip string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, existing := range cm.Data.LANPrinters {
		if existing == ip {
			return nil // Already exists
		}
	}
	cm.Data.LANPrinters = append(cm.Data.LANPrinters, ip)
	return cm.saveLocked()
}

func (cm *Manager) RemoveLANPrinter(ip string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, existing := range cm.Data.LANPrinters {
		if existing == ip {
			cm.Data.LANPrinters = append(cm.Data.LANPrinters[:i], cm.Data.LANPrinters[i+1:]...)
			return cm.saveLocked()
		}
	}
	return nil // Not found, nothing to remove
}

// GetWebViewURL returns the configured kiosk URL.
func (cm *Manager) GetWebViewURL() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewURL
}

// GetWebViewEnabled returns whether kiosk mode is enabled.
func (cm *Manager) GetWebViewEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewEnabled
}

// HasWebViewPIN reports whether a PIN has been configured.
func (cm *Manager) HasWebViewPIN() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewPIN != ""
}

// SetWebViewURL validates and persists the kiosk URL.
func (cm *Manager) SetWebViewURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed != "" {
		parsed, err := url.ParseRequestURI(trimmed)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("invalid URL: must be a valid HTTP or HTTPS address")
		}
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.WebViewURL = trimmed
	return cm.saveLocked()
}

// SetWebViewPIN validates (exactly 4 digits) and persists the plaintext PIN.
func (cm *Manager) SetWebViewPIN(pin string) error {
	if len(pin) != 4 {
		return errors.New("PIN must be exactly 4 digits")
	}
	for _, ch := range pin {
		if ch < '0' || ch > '9' {
			return errors.New("PIN must contain digits only")
		}
	}
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.WebViewPIN = pin
	return cm.saveLocked()
}

// CheckWebViewPIN returns true when raw matches the stored PIN.
func (cm *Manager) CheckWebViewPIN(raw string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.WebViewPIN != "" && cm.Data.WebViewPIN == raw
}

// SetWebViewEnabled persists the enabled flag.
func (cm *Manager) SetWebViewEnabled(v bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if v && cm.Data.WebViewURL == "" {
		return errors.New("cannot enable kiosk mode: URL is not configured")
	}
	cm.Data.WebViewEnabled = v
	return cm.saveLocked()
}

var validCorners = map[string]bool{
	"top-left":     true,
	"top-right":    true,
	"bottom-left":  true,
	"bottom-right": true,
}

// GetWebViewExitCorners returns the configured exit gesture corners.
// Defaults to ["top-right"] if none are configured.
func (cm *Manager) GetWebViewExitCorners() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []string
	seen := make(map[string]bool)

	for _, c := range cm.Data.WebViewExitCorners {
		c = strings.ToLower(strings.TrimSpace(c))
		if validCorners[c] && !seen[c] {
			seen[c] = true
			result = append(result, c)
		}
	}

	if len(result) == 0 {
		return []string{"top-right"}
	}
	return result
}

// SetWebViewExitCorners validates and persists the list of active exit gesture corners.
func (cm *Manager) SetWebViewExitCorners(corners []string) error {
	var valid []string
	seen := make(map[string]bool)

	for _, c := range corners {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if !validCorners[c] {
			return fmt.Errorf("invalid exit corner: %q (must be top-right, top-left, bottom-right, or bottom-left)", c)
		}
		if !seen[c] {
			seen[c] = true
			valid = append(valid, c)
		}
	}

	if len(valid) == 0 {
		return errors.New("at least one exit corner must be selected")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Data.WebViewExitCorners = valid
	return cm.saveLocked()
}

// GetWebViewExitCorner returns a string representation of the configured exit corner(s).
func (cm *Manager) GetWebViewExitCorner() string {
	corners := cm.GetWebViewExitCorners()
	return strings.Join(corners, ",")
}

// SetWebViewExitCorner validates and persists exit gesture corner(s) (supports single or comma-separated).
func (cm *Manager) SetWebViewExitCorner(corner string) error {
	parts := strings.Split(corner, ",")
	return cm.SetWebViewExitCorners(parts)
}

func (cm *Manager) GetLANPrinters() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.Data.LANPrinters == nil {
		return []string{}
	}
	// Return a copy to avoid races if caller modifies the slice
	result := make([]string, len(cm.Data.LANPrinters))
	copy(result, cm.Data.LANPrinters)
	return result
}

// IsKioskEnabled returns whether server/kiosk mode is enabled.
func (cm *Manager) IsKioskEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Data.Kiosk.Enabled
}
