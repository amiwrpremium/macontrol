package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/amiwrpremium/macontrol/internal/config"
	"golang.org/x/term" // added to go.mod via tidy when needed
)

func runSetup(args []string) {
	reconfigure := contains(args, "--reconfigure")
	fmt.Println("macontrol first-run setup. Press Ctrl-C to abort.")
	fmt.Println()

	cfgPath, err := config.DefaultConfigPath()
	if err != nil {
		fatalf("could not derive config path: %v", err)
	}
	if _, statErr := os.Stat(cfgPath); statErr == nil && !reconfigure {
		fmt.Printf("⚠ config already exists at %s\n   run `macontrol setup --reconfigure` to overwrite.\n", cfgPath)
		return
	}

	in := bufio.NewReader(os.Stdin)

	token := promptHidden("▸ Telegram bot token (from @BotFather): ")
	if token == "" {
		fatalf("token is required")
	}
	primary := promptLine(in, "▸ Your Telegram user ID (from @userinfobot): ")
	primary = strings.TrimSpace(primary)
	if _, err := strconv.ParseInt(primary, 10, 64); err != nil {
		fatalf("user id must be an integer, got %q", primary)
	}
	extra := promptLine(in, "▸ Additional user IDs to allow, comma-separated (blank = none): ")
	ids := strings.TrimSpace(primary)
	if extra = strings.TrimSpace(extra); extra != "" {
		ids = ids + "," + extra
	}

	fmt.Print("▸ Verifying token…  ")
	botUser, err := verifyToken(token)
	if err != nil {
		fmt.Println("✗")
		fatalf("token verification failed: %v", err)
	}
	fmt.Printf("✓ bot @%s\n", botUser)

	writeConfig(cfgPath, token, ids)
	fmt.Printf("▸ Writing config to %s  ✓\n", cfgPath)

	installAgent := promptYesNo(in, "▸ Install LaunchAgent so macontrol starts at login? [Y/n] ", true)
	if installAgent {
		if err := serviceInstall(); err != nil {
			fmt.Printf("⚠ could not install LaunchAgent: %v\n", err)
		} else {
			fmt.Println("▸ LaunchAgent installed  ✓")
		}
	}

	installSudoers := promptYesNo(in, "▸ Install narrow sudoers entry (shutdown/pmset/wdutil/powermetrics/systemsetup)? [y/N] ", false)
	if installSudoers {
		if err := installSudoersFile(); err != nil {
			fmt.Printf("⚠ could not install sudoers entry: %v\n", err)
			fmt.Println("  You can install it later by copying sudoers.d/macontrol.sample to /etc/sudoers.d/macontrol via `sudo visudo -f /etc/sudoers.d/macontrol`.")
		} else {
			fmt.Println("▸ /etc/sudoers.d/macontrol written  ✓")
		}
	}

	fmt.Println()
	fmt.Println("TCC permissions to grant (System Settings → Privacy & Security):")
	fmt.Println("  • Screen Recording  — /screenshot, /record")
	fmt.Println("  • Accessibility     — app listing, fallback brightness")
	fmt.Println("  • Camera            — /photo")
	fmt.Println()

	if installAgent && promptYesNo(in, "▸ Start the daemon now? [Y/n] ", true) {
		if err := serviceStart(); err != nil {
			fmt.Printf("⚠ start failed: %v\n", err)
		} else {
			fmt.Println("  daemon started.")
		}
	}
	fmt.Printf("\nDone. Send /start to @%s.\n", botUser)
}

func promptLine(in *bufio.Reader, label string) string {
	fmt.Print(label)
	s, _ := in.ReadString('\n')
	return strings.TrimRight(s, "\r\n")
}

func promptHidden(label string) string {
	fmt.Print(label)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		fatalf("could not read hidden input: %v", err)
	}
	return strings.TrimSpace(string(b))
}

func promptYesNo(in *bufio.Reader, label string, def bool) bool {
	s := promptLine(in, label)
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return def
	}
	return s == "y" || s == "yes"
}

func writeConfig(path, token, ids string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fatalf("mkdir: %v", err)
	}
	body := fmt.Sprintf("TELEGRAM_BOT_TOKEN=%s\nALLOWED_USER_IDS=%s\nLOG_LEVEL=info\n", token, ids)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		fatalf("write config: %v", err)
	}
}

// verifyToken calls getMe via the Bot API to confirm the token is valid.
func verifyToken(token string) (string, error) {
	reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, "GET",
		"https://api.telegram.org/bot"+url.PathEscape(token)+"/getMe", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var parsed struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if !parsed.OK {
		return "", fmt.Errorf("telegram API: %s", parsed.Description)
	}
	return parsed.Result.Username, nil
}

func contains(ss []string, needle string) bool {
	for _, s := range ss {
		if s == needle {
			return true
		}
	}
	return false
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "macontrol setup: "+format+"\n", args...)
	os.Exit(1)
}

// installSudoersFile installs a narrow /etc/sudoers.d/macontrol via
// `sudo tee` + `sudo visudo -cf` to validate.
func installSudoersFile() error {
	content := sudoersBody()
	tmp, err := os.CreateTemp("", "macontrol-sudoers-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		return err
	}
	tmp.Close()

	// Validate locally first.
	check := exec.Command("sudo", "visudo", "-cf", tmp.Name())
	check.Stdin = os.Stdin
	check.Stdout = os.Stdout
	check.Stderr = os.Stderr
	if err := check.Run(); err != nil {
		return fmt.Errorf("visudo check failed: %w", err)
	}
	// Install.
	install := exec.Command("sudo", "install", "-m", "0440", "-o", "root", "-g", "wheel",
		tmp.Name(), "/etc/sudoers.d/macontrol")
	install.Stdin = os.Stdin
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	return install.Run()
}
