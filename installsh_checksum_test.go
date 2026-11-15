package main

// install.sh verifies the download against the release's checksums.txt.
// A wrong sum must stop the install with nothing written; a right one must
// not get in the way; a release with no checksums.txt installs with a warning
// (the first releases never carried one).

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheInstallScriptVerifiesTheDownload(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("no curl")
	}
	if exec.Command("/bin/sh", "-c", "command -v sha256sum || command -v shasum").Run() != nil {
		t.Skip("no sha256sum or shasum")
	}
	dir, asset := buildForInstallTest(t)
	data, err := os.ReadFile(filepath.Join(dir, asset))
	if err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer srv.Close()

	t.Run("a matching sum installs", func(t *testing.T) {
		os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(sum+"  "+asset+"\n"), 0o644)
		r := runInstallScript(t, srv.URL, "/bin/zsh", systemPath, "")
		if r.err != nil {
			t.Fatalf("install.sh: %v\n%s\n%s", r.err, r.stdout, r.stderr)
		}
		if _, err := os.Stat(r.bin); err != nil {
			t.Fatalf("no binary: %v", err)
		}
	})
	t.Run("a wrong sum stops with nothing installed", func(t *testing.T) {
		bad := strings.Repeat("0", 64)
		os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(bad+"  "+asset+"\n"), 0o644)
		r := runInstallScript(t, srv.URL, "/bin/zsh", systemPath, "")
		if r.err == nil {
			t.Fatalf("install.sh accepted a binary whose checksum does not match:\n%s", r.stdout)
		}
		if !strings.Contains(r.stderr, "checksum mismatch") {
			t.Fatalf("the failure does not say why:\n%s", r.stderr)
		}
		if _, err := os.Stat(r.bin); err == nil {
			t.Fatalf("a binary was installed despite the mismatch")
		}
	})
	t.Run("no checksums.txt installs with a warning", func(t *testing.T) {
		os.Remove(filepath.Join(dir, "checksums.txt"))
		r := runInstallScript(t, srv.URL, "/bin/zsh", systemPath, "")
		if r.err != nil {
			t.Fatalf("install.sh: %v\n%s\n%s", r.err, r.stdout, r.stderr)
		}
		if !strings.Contains(r.stdout, "not verified") {
			t.Fatalf("no warning that the download was unverified:\n%s", r.stdout)
		}
	})
}
