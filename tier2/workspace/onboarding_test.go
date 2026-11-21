package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicOnboardingAndReleases(t *testing.T) {
	t.Setenv("KALKI_PUBLIC_TLS", "true")
	f := setup(t)
	session := login(t, f.aps, "tenant_alpha")
	commands := mint(t, f.aps, session, "", false)
	for _, key := range []string{"install_command", "collector_command", "windows_install_command"} {
		cmd, ok := commands[key].(string)
		if !ok {
			t.Fatal("missing", key)
		}
		if strings.Contains(cmd, "ca.pem") || strings.Contains(cmd, "127.0.0.1:1") || strings.Contains(cmd, "?token=") {
			t.Fatal("unsafe or unusable public command", key)
		}
		if !strings.Contains(cmd, "2s") {
			t.Fatal("missing fast window")
		}
	}
	for _, file := range []string{"collector-linux-amd64", "collector-linux-arm64", "collector-darwin-amd64", "collector-darwin-arm64", "collector-windows-amd64.exe", "allowlist.yaml"} {
		if err := os.WriteFile(filepath.Join(f.dir, file), []byte("test-release"), 0600); err != nil {
			t.Fatal(err)
		}
		code, body := request(t, f.aps, "GET", "/downloads/"+file, "", nil, "")
		if code != 200 || string(body) != "test-release" {
			t.Fatalf("download %s: %d", file, code)
		}
	}
	for _, path := range []string{"/install.sh", "/install.ps1"} {
		code, body := request(t, f.aps, "GET", path, "", nil, "")
		if code != 200 || len(body) < 100 {
			t.Fatalf("installer %s: %d", path, code)
		}
		if strings.Contains(string(body), commands["token"].(string)) {
			t.Fatal("token embedded in installer")
		}
	}
}

func TestWindowsInstallerParses(t *testing.T) {
	command, err := exec.LookPath("powershell")
	if err != nil {
		command, err = exec.LookPath("pwsh")
	}
	if err != nil {
		t.Skip("PowerShell parser unavailable on this OS")
	}
	cmd := exec.Command(command, "-NoProfile", "-NonInteractive", "-Command", `$source=[Console]::In.ReadToEnd(); $parseErrors=$null; $tokens=$null; [void][System.Management.Automation.Language.Parser]::ParseInput($source,[ref]$tokens,[ref]$parseErrors); if($parseErrors.Count){exit 1}`)
	cmd.Stdin = strings.NewReader(windowsInstaller)
	if err = cmd.Run(); err != nil {
		t.Fatal("PowerShell installer syntax invalid")
	}
}
