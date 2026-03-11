package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

const (
	repo         = "Synergix-lab/WRAI.TH"
	releaseAPI   = "https://api.github.com/repos/" + repo + "/releases/latest"
	serviceLabel = "com.agent-relay"
	binaryName   = "agent-relay"
)

func runUpdate(args []string) {
	force := false
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		}
		if a == "--help" || a == "-h" {
			fmt.Print(`usage: agent-relay update [--force]

Check for updates and install the latest version.

flags:
  -f, --force   Update even if already on latest version
  -h, --help    Show this help
`)
			return
		}
	}

	// 1. Get current version
	currentVersion := getCurrentVersion()
	fmt.Printf("  current: %s\n", currentVersion)

	// 2. Get latest version from GitHub
	fmt.Print("  checking latest release... ")
	latestVersion, err := getLatestVersion()
	if err != nil {
		fmt.Printf("\nerror: could not check latest version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", latestVersion)

	// 3. Compare
	if !force && currentVersion == latestVersion {
		fmt.Println("\n  already up to date")
		return
	}

	if !force {
		fmt.Printf("\n  update available: %s → %s\n", currentVersion, latestVersion)
	}

	// 4. Find the binary path
	binPath, err := findBinaryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  binary: %s\n", binPath)

	// 5. Try build from source, fallback to download
	if tryBuildUpdate(binPath, latestVersion) {
		fmt.Println("  built from source")
	} else if tryDownloadUpdate(binPath, latestVersion) {
		fmt.Println("  downloaded prebuilt binary")
	} else {
		fmt.Fprintln(os.Stderr, "error: update failed — could not build or download")
		os.Exit(1)
	}

	// 6. Restart service
	restartService()

	// 7. Verify
	fmt.Print("\n  verifying... ")
	newVersion := getCurrentVersion()
	fmt.Printf("%s\n", newVersion)
	fmt.Println("\n  update complete")
}

func getCurrentVersion() string {
	out, err := exec.Command(binaryName, "--version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	v := strings.TrimSpace(string(out))
	// Output is "agent-relay v0.3.1" → extract version
	if parts := strings.Fields(v); len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return v
}

func getLatestVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(releaseAPI)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no releases found")
	}
	return release.TagName, nil
}

func findBinaryPath() (string, error) {
	// Check if we're in the source repo
	if _, err := os.Stat("go.mod"); err == nil {
		if data, err := os.ReadFile("go.mod"); err == nil && strings.Contains(string(data), "agent-relay") {
			// We're in the source repo — build in place
			exe, _ := os.Executable()
			if exe != "" {
				return exe, nil
			}
		}
	}

	// Find installed binary
	path, err := exec.LookPath(binaryName)
	if err == nil {
		// Resolve symlinks
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return resolved, nil
		}
		return path, nil
	}

	// Common paths
	for _, p := range []string{
		"/usr/local/bin/" + binaryName,
		filepath.Join(os.Getenv("HOME"), ".local", "bin", binaryName),
	} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("cannot find %s binary", binaryName)
}

func tryBuildUpdate(binPath, version string) bool {
	// Need go and a C compiler
	if _, err := exec.LookPath("go"); err != nil {
		return false
	}
	hasCc := false
	for _, cc := range []string{"cc", "gcc", "clang"} {
		if _, err := exec.LookPath(cc); err == nil {
			hasCc = true
			break
		}
	}
	if !hasCc {
		return false
	}

	// Clone to temp dir
	tmpDir, err := os.MkdirTemp("", "agent-relay-update-*")
	if err != nil {
		return false
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	fmt.Print("  cloning... ")
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", version,
		"https://github.com/"+repo+".git", filepath.Join(tmpDir, "src"))
	if err := cmd.Run(); err != nil {
		// Try without --branch (tag might not exist for dev)
		cmd = exec.Command("git", "clone", "--depth", "1",
			"https://github.com/"+repo+".git", filepath.Join(tmpDir, "src"))
		if err := cmd.Run(); err != nil {
			fmt.Println("failed")
			return false
		}
	}
	fmt.Println("ok")

	fmt.Print("  building... ")
	buildCmd := exec.Command("go", "build", "-tags", "fts5",
		"-ldflags", fmt.Sprintf("-s -w -X main.Version=%s", version),
		"-o", filepath.Join(tmpDir, binaryName), ".")
	buildCmd.Dir = filepath.Join(tmpDir, "src")
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	if err := buildCmd.Run(); err != nil {
		fmt.Println("failed")
		return false
	}
	fmt.Println("ok")

	// Replace binary
	return installBinary(filepath.Join(tmpDir, binaryName), binPath)
}

func tryDownloadUpdate(binPath, version string) bool {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	archiveName := fmt.Sprintf("agent-relay-%s-%s.tar.gz", osName, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, archiveName)

	tmpDir, err := os.MkdirTemp("", "agent-relay-dl-*")
	if err != nil {
		return false
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	fmt.Print("  downloading... ")
	archivePath := filepath.Join(tmpDir, archiveName)
	cmd := exec.Command("curl", "-fsSL", "-o", archivePath, url)
	if err := cmd.Run(); err != nil {
		fmt.Println("failed")
		return false
	}
	fmt.Println("ok")

	// Extract
	cmd = exec.Command("tar", "-xzf", archivePath, "-C", tmpDir)
	if err := cmd.Run(); err != nil {
		return false
	}

	return installBinary(filepath.Join(tmpDir, binaryName), binPath)
}

func installBinary(src, dst string) bool {
	// Try direct copy first
	cmd := exec.Command("install", "-m", "755", src, dst)
	if err := cmd.Run(); err != nil {
		// Try with sudo
		cmd = exec.Command("sudo", "install", "-m", "755", src, dst)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "  error: could not install binary to %s: %v\n", dst, err)
			return false
		}
	}
	return true
}

func restartService() {
	fmt.Print("  restarting service... ")

	switch runtime.GOOS {
	case "darwin":
		plist := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", serviceLabel+".plist")
		if _, err := os.Stat(plist); err != nil {
			fmt.Println("no service found (manual start)")
			return
		}
		uid := os.Getuid()
		// Stop
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", uid), plist).Run()
		// Small delay for clean shutdown
		time.Sleep(500 * time.Millisecond)
		// Start
		if err := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), plist).Run(); err != nil {
			_ = exec.Command("launchctl", "load", plist).Run()
		}
		fmt.Println("ok (launchd)")

	case "linux":
		// Check if systemd service exists
		if err := exec.Command("systemctl", "--user", "is-enabled", binaryName).Run(); err != nil {
			fmt.Println("no service found (manual start)")
			return
		}
		_ = exec.Command("systemctl", "--user", "restart", binaryName).Run()
		fmt.Println("ok (systemd)")

	default:
		fmt.Println("unsupported platform — restart manually")
	}
}
