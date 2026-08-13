package appsvc_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"lrss/internal/appsvc"
	"lrss/internal/autostart"
	"lrss/internal/db"
	"lrss/internal/job"
	"lrss/internal/search"
	"lrss/internal/settings"
)

func newSettingsService(t *testing.T) *appsvc.SettingsService {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	searchSvc := search.New(database.SQL, store)
	embedWorker := job.NewEmbedWorker(database.SQL, store)
	return appsvc.NewSettings(store, searchSvc, embedWorker)
}

func isolateLaunchAtLogin(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		name := fmt.Sprintf("LRSS-test-%d-%d", os.Getpid(), time.Now().UnixNano())
		restore := autostart.SetValueNameForTest(name)
		t.Cleanup(restore)
		t.Cleanup(func() { _ = autostart.Set(false) })
	case "darwin", "linux":
		restore := autostart.SetHomeDirForTest(t.TempDir())
		t.Cleanup(restore)
	}
}

func TestSettingsService_LaunchAtLogin(t *testing.T) {
	svc := newSettingsService(t)
	isolateLaunchAtLogin(t)

	st, err := svc.GetLaunchAtLogin()
	if err != nil {
		t.Fatal(err)
	}
	if st.Supported != autostart.Supported() {
		t.Fatalf("Supported = %v want %v (err=%q)", st.Supported, autostart.Supported(), st.Error)
	}

	if !autostart.Supported() {
		_, err := svc.SetLaunchAtLogin(true)
		if err == nil {
			t.Fatal("SetLaunchAtLogin on unsupported OS should error")
		}
		return
	}

	st, err = svc.SetLaunchAtLogin(true)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Supported {
		t.Fatal("expected supported")
	}
	if !st.Enabled {
		t.Fatalf("expected enabled after Set(true): %+v", st)
	}

	st, err = svc.GetLaunchAtLogin()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Enabled {
		t.Fatalf("Get after Set(true): %+v", st)
	}

	st, err = svc.SetLaunchAtLogin(false)
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled {
		t.Fatalf("expected disabled after Set(false): %+v", st)
	}
}
