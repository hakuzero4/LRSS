package appsvc

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"lrss/internal/update"
)

// UpdateService checks GitHub Releases and can download+install for this platform.
type UpdateService struct {
	Version string
	Owner   string
	Repo    string

	mu   sync.Mutex
	busy bool

	// Quit is called after a successful install schedule so the updater script can replace files.
	//
	//wails:ignore
	Quit func()
}

// NewUpdate constructs the service. version is the running app semver (no "v" prefix ok).
func NewUpdate(version string) *UpdateService {
	return &UpdateService{
		Version: version,
		Owner:   update.DefaultOwner,
		Repo:    update.DefaultRepo,
	}
}

// SetQuit injects the application quit callback.
//
//wails:ignore
func (u *UpdateService) SetQuit(fn func()) {
	u.Quit = fn
}

func (u *UpdateService) client() *update.Client {
	return &update.Client{
		Owner:      u.Owner,
		Repo:       u.Repo,
		AppVersion: u.Version,
	}
}

// CheckForUpdate returns latest release status for this platform.
func (u *UpdateService) CheckForUpdate() (update.CheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return u.client().Check(ctx), nil
}

// DownloadAndInstall downloads the matching release asset, schedules replace-on-exit, then quits.
// The frontend should show a short "restarting" message; the process will exit shortly after.
func (u *UpdateService) DownloadAndInstall() (update.InstallResult, error) {
	u.mu.Lock()
	if u.busy {
		u.mu.Unlock()
		return update.InstallResult{OK: false, Message: "busy"}, fmt.Errorf("busy")
	}
	u.busy = true
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.busy = false
		u.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	res, err := u.client().DownloadAndSchedule(ctx)
	if err != nil {
		return res, err
	}
	if res.OK {
		log.Printf("update scheduled: latest=%s asset=%s — quitting for replace", res.Latest, res.Asset)
		// Quit after returning so the RPC can complete.
		go func() {
			time.Sleep(800 * time.Millisecond)
			if u.Quit != nil {
				u.Quit()
			}
		}()
	}
	return res, nil
}
