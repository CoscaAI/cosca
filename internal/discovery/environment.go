package discovery

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/term"
)

// EnvironmentInfo contains all discovered environment information.
type EnvironmentInfo struct {
	OS             string `json:"os" yaml:"os"`
	OSVersion      string `json:"os_version,omitempty" yaml:"os_version,omitempty"`
	Architecture   string `json:"architecture" yaml:"architecture"`
	Shell          string `json:"shell,omitempty" yaml:"shell,omitempty"`
	ShellVersion   string `json:"shell_version,omitempty" yaml:"shell_version,omitempty"`
	Terminal       string `json:"terminal,omitempty" yaml:"terminal,omitempty"`
	TerminalWidth  int    `json:"terminal_width,omitempty" yaml:"terminal_width,omitempty"`
	TerminalHeight int    `json:"terminal_height,omitempty" yaml:"terminal_height,omitempty"`
	CPUCount       int    `json:"cpu_count" yaml:"cpu_count"`
	MemoryTotal    int64  `json:"memory_total_bytes,omitempty" yaml:"memory_total_bytes,omitempty"`
	MemoryFree     int64  `json:"memory_free_bytes,omitempty" yaml:"memory_free_bytes,omitempty"`
	DiskTotal      int64  `json:"disk_total_bytes,omitempty" yaml:"disk_total_bytes,omitempty"`
	DiskFree       int64  `json:"disk_free_bytes,omitempty" yaml:"disk_free_bytes,omitempty"`
	IsWSL          bool   `json:"is_wsl" yaml:"is_wsl"`
	HasGPU         bool   `json:"has_gpu" yaml:"has_gpu"`
	GPUInfo        string `json:"gpu_info,omitempty" yaml:"gpu_info,omitempty"`
	NetworkOnline  bool   `json:"network_online" yaml:"network_online"`
	IsCI           bool   `json:"is_ci" yaml:"is_ci"`
	IsDocker       bool   `json:"is_docker" yaml:"is_docker"`
	HomeDir        string `json:"home_dir" yaml:"home_dir"`
	TempDir        string `json:"temp_dir" yaml:"temp_dir"`
	Hostname       string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	Username       string `json:"username,omitempty" yaml:"username,omitempty"`
}

// DetectEnvironment detects the full environment information.
func DetectEnvironment(_ context.Context, logger zerolog.Logger) (*EnvironmentInfo, error) {
	info := &EnvironmentInfo{}

	// OS information
	info.OS = runtime.GOOS
	info.Architecture = runtime.GOARCH
	info.CPUCount = runtime.NumCPU()

	// OS version
	info.OSVersion = detectOSVersion()

	// Detect WSL
	info.IsWSL = detectWSL()

	// Detect Docker
	info.IsDocker = detectDockerEnv()

	// Detect CI
	info.IsCI = detectCI()

	// Shell detection
	info.Shell, info.ShellVersion = detectShell()

	// Terminal detection
	info.Terminal = detectTerminal()

	// Terminal size
	if term.IsTerminal(int(os.Stdout.Fd())) {
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err == nil {
			info.TerminalWidth = width
			info.TerminalHeight = height
		}
	}

	// System info
	info.HomeDir, _ = os.UserHomeDir()
	info.TempDir = os.TempDir()
	info.Hostname, _ = os.Hostname()
	info.Username = os.Getenv("USER")
	if info.Username == "" {
		info.Username = os.Getenv("USERNAME")
	}

	// Memory info
	info.MemoryTotal, info.MemoryFree = detectMemory()

	// Disk info
	info.DiskTotal, info.DiskFree = detectDisk()

	// GPU detection
	info.HasGPU, info.GPUInfo = detectGPU()

	// Network detection
	info.NetworkOnline = detectNetwork()

	logger.Debug().
		Str("os", info.OS).
		Str("arch", info.Architecture).
		Str("shell", info.Shell).
		Str("terminal", info.Terminal).
		Int("cpus", info.CPUCount).
		Bool("wsl", info.IsWSL).
		Bool("docker", info.IsDocker).
		Bool("ci", info.IsCI).
		Bool("gpu", info.HasGPU).
		Bool("network", info.NetworkOnline).
		Msg("environment discovery complete")

	return info, nil
}

// detectOSVersion detects the OS version.
func detectOSVersion() string {
	switch runtime.GOOS {
	case "linux":
		// Try /etc/os-release first
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
				}
			}
		}
		// Fallback to uname
		if output, err := exec.Command("uname", "-r").Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	case "darwin":
		if output, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			return "macOS " + strings.TrimSpace(string(output))
		}
	case "windows":
		return os.Getenv("OS")
	}
	return runtime.GOOS
}

// detectWSL checks if running under WSL.
func detectWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft") ||
		strings.Contains(strings.ToLower(string(data)), "wsl")
}

// detectDockerEnv checks if running inside a Docker container.
func detectDockerEnv() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "docker")
}

// detectCI checks if running in a CI environment.
func detectCI() bool {
	ciEnvVars := []string{
		"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL",
		"CIRCLECI", "TRAVIS", "BUILDKITE", "CODEBUILD",
		"TF_BUILD", "BITBUCKET_BUILD_NUMBER",
	}
	for _, env := range ciEnvVars {
		if os.Getenv(env) != "" {
			return true
		}
	}
	return false
}

// detectShell detects the current shell.
func detectShell() (string, string) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = os.Getenv("COMSPEC")
	}
	if shell == "" {
		return "unknown", ""
	}

	// Extract shell name and check version
	shellName := filepath.Base(shell)

	var version string
	switch shellName {
	case "bash":
		if output, err := exec.Command(shell, "--version").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) > 0 {
				parts := strings.Fields(lines[0])
				for i, p := range parts {
					if p == "version" && i+1 < len(parts) {
						version = parts[i+1]
						break
					}
				}
			}
		}
	case "zsh":
		if output, err := exec.Command(shell, "--version").Output(); err == nil {
			parts := strings.Fields(string(output))
			if len(parts) >= 2 {
				version = strings.TrimRight(parts[len(parts)-1], ")")
			}
		}
	case "fish":
		if output, err := exec.Command(shell, "--version").Output(); err == nil {
			parts := strings.Fields(string(output))
			if len(parts) >= 3 {
				version = parts[2]
			}
		}
	}

	return shellName, version
}

// detectTerminal detects the terminal emulator.
func detectTerminal() string {
	// Check common terminal env vars in order of specificity
	termProg := os.Getenv("TERM_PROGRAM")
	if termProg != "" {
		version := os.Getenv("TERM_PROGRAM_VERSION")
		if version != "" {
			return termProg + " " + version
		}
		return termProg
	}

	term := os.Getenv("TERM")
	if term != "" && term != "xterm" && term != "xterm-256color" {
		return term
	}

	// Check platform-specific terminals
	switch {
	case os.Getenv("KONSOLE_VERSION") != "":
		return "Konsole"
	case os.Getenv("GNOME_TERMINAL_SERVICE") != "":
		return "GNOME Terminal"
	case os.Getenv("ITERM_SESSION_ID") != "":
		return "iTerm2"
	case os.Getenv("APPLE_TERMINAL") != "":
		return "Apple Terminal"
	case os.Getenv("ALACRITTY_LOG") != "":
		return "Alacritty"
	case os.Getenv("KITTY_PID") != "":
		return "Kitty"
	case os.Getenv("WEZTERM_PANE") != "":
		return "WezTerm"
	case os.Getenv("TMUX") != "":
		return "tmux"
	case os.Getenv("STY") != "":
		return "screen"
	}

	return "unknown"
}

// detectMemory detects total and free system memory.
func detectMemory() (total int64, free int64) {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0, 0
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				var kb int64
				_, _ = fmt.Sscanf(line, "MemTotal: %d kB", &kb)
				total = kb * 1024
			} else if strings.HasPrefix(line, "MemAvailable:") {
				var kb int64
				_, _ = fmt.Sscanf(line, "MemAvailable: %d kB", &kb)
				free = kb * 1024
			}
		}
	case "darwin":
		if output, err := exec.Command("sysctl", "hw.memsize").Output(); err == nil {
			_, _ = fmt.Sscanf(string(output), "hw.memsize: %d", &total)
		}
		free = total / 4 // rough estimate
	}

	return total, free
}

// detectDisk detects total and free disk space for the current directory.
func detectDisk() (total int64, free int64) {
	switch runtime.GOOS {
	case "linux":
		if output, err := exec.Command("df", "-B1", ".").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 4 {
					_, _ = fmt.Sscanf(fields[1], "%d", &total)
					_, _ = fmt.Sscanf(fields[3], "%d", &free)
				}
			}
		}
	case "darwin":
		if output, err := exec.Command("df", "-k", ".").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 4 {
					var totalK, freeK int64
					_, _ = fmt.Sscanf(fields[1], "%d", &totalK)
					_, _ = fmt.Sscanf(fields[3], "%d", &freeK)
					total = totalK * 1024
					free = freeK * 1024
				}
			}
		}
	}

	return total, free
}

// detectGPU checks for GPU availability.
func detectGPU() (bool, string) {
	// Check for NVIDIA GPU
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		if output, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
			gpuName := strings.TrimSpace(string(output))
			return true, "NVIDIA: " + gpuName
		}
	}

	// Check for AMD GPU
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		return true, "AMD ROCm"
	}

	// Check for Apple Silicon
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			if strings.Contains(string(output), "Apple") {
				return true, "Apple Silicon"
			}
		}
	}

	// Check for GPU via /dev/dri on Linux
	if runtime.GOOS == "linux" {
		if entries, err := os.ReadDir("/dev/dri"); err == nil && len(entries) > 0 {
			return true, "DRM device available"
		}
	}

	return false, ""
}

// detectNetwork checks if network is likely available.
func detectNetwork() bool {
	if os.Getenv("COSCA_OFFLINE") != "" {
		return false
	}

	// Basic check - if we can access /sys/class/net, assume online
	_, err := os.Stat("/sys/class/net")
	if err != nil {
		return true // Assume online if can't check
	}

	return true
}
