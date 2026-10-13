package autostart

import (
	"os"
	"strings"
	"testing"
)

func TestLinuxAutostartEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if Enabled() {
		t.Fatal("enabled before Set")
	}
	if err := Set(true, "/opt/my apps/odm"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(entryPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `Exec="/opt/my apps/odm" -hidden`) || !Enabled() {
		t.Fatalf("entry:\n%s", data)
	}
	if err := Set(false, ""); err != nil || Enabled() {
		t.Fatalf("disable: %v, still enabled=%v", err, Enabled())
	}
	if err := Set(false, ""); err != nil {
		t.Fatalf("disabling twice should be fine: %v", err)
	}
}
