package logic

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type OSInfo struct {
	OS            string `json:"os"`             // e.g. "ubuntu", "windows", "macos"
	OSFamily      string `json:"os_family"`      // e.g. "linux-debian", "windows"
	OSVersion     string `json:"os_version"`     // e.g. "22.04", "10.0.22631"
	KernelVersion string `json:"kernel_version"` // e.g. "6.8.0"
}

/// --- classification helpers you already had ---

func NormalizeOSName(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// OSFamily returns a normalized OS family string.
// Examples: "linux-debian", "linux-redhat", "linux-arch", "linux-other", "windows", "darwin"
func OSFamily(osName string) string {
	osName = NormalizeOSName(osName)

	// Non-Linux first
	if strings.Contains(osName, "windows") {
		return "windows"
	}
	if strings.Contains(osName, "darwin") || strings.Contains(osName, "mac") || strings.Contains(osName, "os x") {
		return "darwin"
	}

	// Linux families
	switch {
	// Debian family
	case containsAny(osName,
		"debian", "ubuntu", "pop", "linuxmint", "kali", "raspbian", "elementary"):
		return "linux-debian"

	// Red Hat family
	case containsAny(osName,
		"rhel", "red hat", "centos", "rocky", "alma", "fedora", "oracle linux", "ol"):
		return "linux-redhat"

	// SUSE family
	case containsAny(osName,
		"suse", "opensuse", "sles"):
		return "linux-suse"

	// Arch family
	case containsAny(osName,
		"arch", "manjaro", "endeavouros", "garuda"):
		return "linux-arch"

	// Gentoo
	case strings.Contains(osName, "gentoo"):
		return "linux-gentoo"

	// Alpine, Amazon, BusyBox, etc.
	case containsAny(osName,
		"alpine", "amazon", "busybox"):
		return "linux-other"
	}

	// Fallbacks
	if strings.Contains(osName, "linux") {
		return "linux-other"
	}

	return "unknown"
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

/// --- public entrypoint ---

// GetOSInfo returns OS, OSFamily, OSVersion and KernelVersion for the current platform.
func GetOSInfo() OSInfo {
	switch runtime.GOOS {
	case "linux":
		return getLinuxOSInfo()
	case "darwin":
		return getDarwinOSInfo()
	case "windows":
		return getWindowsOSInfo()
	default:
		// Fallback for other UNIX-likes; best-effort
		kernel := strings.TrimSpace(runCmd("uname", "-r"))
		name := runtime.GOOS
		return OSInfo{
			OS:            NormalizeOSName(name),
			OSFamily:      OSFamily(name),
			OSVersion:     "",
			KernelVersion: CleanVersion(kernel),
		}
	}
}

/// --- Linux ---

func getLinuxOSInfo() OSInfo {
	var osName, osVersion string

	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := parts[0]
			value := strings.Trim(parts[1], `"'`)

			switch key {
			case "ID":
				osName = value
			case "VERSION_ID":
				osVersion = value
			}
		}
	}

	if osName == "" {
		// Fallback
		osName = "linux"
	}
	kernel := strings.TrimSpace(runCmd("uname", "-r"))
	// trim extras like -generic
	if idx := strings.Index(kernel, "-"); idx > 0 {
		kernel = kernel[:idx]
	}

	normName := NormalizeOSName(osName)
	return OSInfo{
		OS:            "linux",
		OSFamily:      OSFamily(normName),
		OSVersion:     CleanVersion(osVersion),
		KernelVersion: CleanVersion(kernel),
	}
}

/// --- macOS (darwin) ---

func getDarwinOSInfo() OSInfo {
	productName := strings.TrimSpace(runCmd("sw_vers", "-productName"))
	productVer := strings.TrimSpace(runCmd("sw_vers", "-productVersion"))

	if productName == "" {
		productName = "macos"
	}
	kernel := strings.TrimSpace(runCmd("uname", "-r"))
	if idx := strings.Index(kernel, "-"); idx > 0 {
		kernel = kernel[:idx]
	}

	normName := NormalizeOSName(productName)
	return OSInfo{
		OS:            "darwin",
		OSFamily:      OSFamily(normName),       // "darwin"
		OSVersion:     CleanVersion(productVer), // e.g. "15.0"
		KernelVersion: CleanVersion(kernel),
	}
}

/// --- Windows ---

func getWindowsOSInfo() OSInfo {
	// OS name: we just say "windows"
	osName := "windows"

	// OS version via "wmic" or "ver" as fallback
	var version string

	// Try wmic first (may be missing on newer builds but often still present)
	out := runCmd("wmic", "os", "get", "Version", "/value")
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Version=") {
			version = strings.TrimPrefix(line, "Version=")
			version = strings.TrimSpace(version)
			break
		}
	}

	if version == "" {
		// Fallback to "ver"
		raw := strings.TrimSpace(runCmd("cmd", "/C", "ver"))
		version = raw // you can add better parsing if you need
	}

	// On Windows, kernel and OS version are effectively tied; reuse
	kernel := version

	normName := NormalizeOSName(osName)
	return OSInfo{
		OS:            "windows", // "windows"
		OSFamily:      OSFamily(normName),
		OSVersion:     CleanVersion(version), // e.g. "10.0.22631"
		KernelVersion: CleanVersion(kernel),
	}
}

/// --- small helper to run commands safely ---

func runCmd(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	_ = cmd.Run() // ignore error; best-effort
	return buf.String()
}
