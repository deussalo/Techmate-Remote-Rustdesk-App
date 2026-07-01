package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	serverIP    = "66.42.121.165"
	publicKey   = "Q7G6CVPOmAYm16QhC0iC7qMeBJdiqvXByYfO02aNIIQ="
	userAgent   = "Techmate-RustDesk"
	downloadDir = "TechmateRemote"
	exeName     = "rustdesk.exe"
)

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type progressWriter struct {
	total       int64
	written     int64
	lastPercent int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.written += int64(n)

	if w.total <= 0 {
		fmt.Printf("\r  Downloaded %d bytes...", w.written)
		return n, nil
	}

	percent := (w.written * 100) / w.total
	if percent != w.lastPercent {
		w.lastPercent = percent
		barLength := int64(40)
		filled := (barLength * percent) / 100
		bar := strings.Repeat("=", int(filled)) + strings.Repeat(" ", int(barLength-filled))
		fmt.Printf("\r  Downloading: [%s] %d%%", bar, percent)
	}

	return n, nil
}

func main() {
	if runtime.GOOS != "windows" {
		fail("This launcher is intended to run on Windows.")
	}

	clearScreen()
	printBanner()

	baseFolder := filepath.Join(os.TempDir(), downloadDir)
	exePath := filepath.Join(baseFolder, exeName)
	configPath, err := rustDeskConfigPath()
	if err != nil {
		fail(err.Error())
	}

	if err := os.MkdirAll(baseFolder, 0o755); err != nil {
		fail(fmt.Sprintf("Could not prepare download folder: %v", err))
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		fail(fmt.Sprintf("Could not prepare config folder: %v", err))
	}

	releaseInfo, err := fetchLatestRelease()
	if err != nil {
		fail(fmt.Sprintf("Could not check the latest RustDesk release: %v", err))
	}

	assetInfo, err := findWindowsAsset(releaseInfo)
	if err != nil {
		fail(err.Error())
	}

	fmt.Println("  Hi, please wait while the secure remote desktop")
	fmt.Println("  software is downloaded.")
	fmt.Println()
	fmt.Printf("  Downloading RustDesk %s\n", releaseInfo.TagName)
	fmt.Println()

	if err := downloadFile(assetInfo.URL, exePath); err != nil {
		fail(fmt.Sprintf("Download failed: %v", err))
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("  Download complete!")

	if err := writeConfig(configPath); err != nil {
		fail(fmt.Sprintf("Could not write RustDesk config: %v", err))
	}

	fmt.Println()
	fmt.Println("  Starting secure connection...")
	time.Sleep(time.Second)

	if err := exec.Command(exePath).Start(); err != nil {
		fail(fmt.Sprintf("Could not start RustDesk: %v", err))
	}

	fmt.Println()
	fmt.Println("  ================================================")
	fmt.Println("       Ready! Please read Arian the ID number")
	fmt.Println("       shown in the RustDesk window.")
	fmt.Println("  ================================================")
	fmt.Println()
	waitForEnter()
}

func fetchLatestRelease() (release, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/rustdesk/rustdesk/releases/latest", nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub returned %s", resp.Status)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return release{}, err
	}
	return rel, nil
}

func findWindowsAsset(rel release) (asset, error) {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".exe") &&
			strings.Contains(a.Name, "x86_64") &&
			!strings.Contains(a.Name, "server") &&
			!strings.HasSuffix(a.Name, ".msi") {
			return a, nil
		}
	}
	return asset{}, errors.New("Could not find a Windows RustDesk download. Please contact Arian.")
}

func downloadFile(url, dest string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	progress := &progressWriter{total: resp.ContentLength, lastPercent: -1}
	if _, err := io.Copy(out, io.TeeReader(resp.Body, progress)); err != nil {
		return err
	}
	return out.Close()
}

func rustDeskConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", errors.New("APPDATA is not set")
	}
	return filepath.Join(appData, "RustDesk", "config", "RustDesk2.toml"), nil
}

func writeConfig(path string) error {
	config := fmt.Sprintf(`rendezvous_server = '%s'
nat_type = 1
serial = 0

[options]
custom-rendezvous-server = '%s'
key = '%s'
`, serverIP, serverIP, publicKey)

	return os.WriteFile(path, []byte(config), 0o644)
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

func printBanner() {
	fmt.Println()
	fmt.Println("  ================================================")
	fmt.Println("           Techmate Remote Support")
	fmt.Println("  ================================================")
	fmt.Println()
}

func waitForEnter() {
	fmt.Println("  Press Enter to close this window...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func fail(message string) {
	fmt.Println()
	fmt.Printf("  %s\n", message)
	fmt.Println()
	waitForEnter()
	os.Exit(1)
}
