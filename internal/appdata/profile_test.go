package appdata

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIsDev_DefaultWithoutProductionTag(t *testing.T) {
	t.Setenv(EnvProfile, "")
	if !IsDev() {
		t.Fatal("go test / wails3 task dev must default to the dev profile")
	}
	if DirName() != dirDev || AppName() != "LRSS Dev" || DisplayName() != "LRSS (dev)" {
		t.Fatalf("dev names: dir=%s app=%s display=%s", DirName(), AppName(), DisplayName())
	}
	if DarwinLabel() != "com.lrss.app.dev" || WindowsRunValue() != "LRSS-dev" {
		t.Fatalf("dev autostart: %s %s", DarwinLabel(), WindowsRunValue())
	}
}

func TestIsDev_EnvOverride(t *testing.T) {
	t.Setenv(EnvProfile, "prod")
	if IsDev() || DirName() != dirProd || AppName() != "LRSS" {
		t.Fatalf("LRSS_PROFILE=prod still looks like dev: %s %s", DirName(), AppName())
	}
	t.Setenv(EnvProfile, "dev")
	if !IsDev() || DirName() != dirDev {
		t.Fatal("LRSS_PROFILE=dev should force the dev profile")
	}
}

func TestDataDir_Override(t *testing.T) {
	want := t.TempDir()
	t.Setenv(EnvDataDir, want)
	got, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("DataDir = %q want %q", got, want)
	}
}

func TestDataDir_ProfileFolder(t *testing.T) {
	t.Setenv(EnvDataDir, "")
	t.Setenv(EnvProfile, "dev")
	got, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.ToSlash(got), dirDev) {
		t.Fatalf("dev DataDir should contain %s: %s", dirDev, got)
	}
}

func TestWebViewUserDataDir(t *testing.T) {
	t.Setenv(EnvProfile, "prod")
	got, err := WebViewUserDataDir()
	if err != nil || got != "" {
		t.Fatalf("prod webview dir = %q err=%v (want empty)", got, err)
	}
	t.Setenv(EnvProfile, "dev")
	got, err = WebViewUserDataDir()
	if err != nil || !strings.Contains(filepath.ToSlash(got), dirDev) {
		t.Fatalf("dev webview dir = %q err=%v", got, err)
	}
}
