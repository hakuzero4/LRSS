//go:build windows

package autostart

import (
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func isolateWindowsRunValue(t *testing.T) string {
	t.Helper()
	name := fmt.Sprintf("LRSS-test-%d-%d", os.Getpid(), time.Now().UnixNano())
	restore := SetValueNameForTest(name)
	t.Cleanup(restore)
	t.Cleanup(func() { _ = Set(false) })
	return name
}

func TestWindowsSetEnabled_IsolatedValue(t *testing.T) {
	name := isolateWindowsRunValue(t)

	on, err := Enabled()
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("test value should not exist yet")
	}

	if err := Set(true); err != nil {
		t.Fatal(err)
	}
	on, err = Enabled()
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("expected enabled after Set(true)")
	}

	val := mustReadRunValue(t, name)
	exe, err := resolveExecutable()
	if err != nil {
		t.Fatal(err)
	}
	if val != quotePath(exe) {
		t.Fatalf("Run value = %q want %q", val, quotePath(exe))
	}

	if err := Set(false); err != nil {
		t.Fatal(err)
	}
	if err := Set(false); err != nil {
		t.Fatalf("second Set(false): %v", err)
	}
	on, err = Enabled()
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("expected disabled after Set(false)")
	}
}

func TestWindowsSet_QuotesPathWithSpaces(t *testing.T) {
	name := isolateWindowsRunValue(t)
	fake := `C:\Program Files\LRSS\lrss.exe`
	prev := resolveExe
	resolveExe = func() (string, error) { return fake, nil }
	t.Cleanup(func() { resolveExe = prev })

	if err := Set(true); err != nil {
		t.Fatal(err)
	}
	val := mustReadRunValue(t, name)
	want := `"C:\Program Files\LRSS\lrss.exe"`
	if val != want {
		t.Fatalf("Run value = %q want %q", val, want)
	}
}

func mustReadRunValue(t *testing.T, name string) string {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, windowsRunKeyPath, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	val, _, err := k.GetStringValue(name)
	if err != nil {
		t.Fatal(err)
	}
	return val
}
