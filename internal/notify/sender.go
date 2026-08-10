// Package notify sends desktop notifications for library events.
package notify

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"lrss/internal/settings"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// Sender delivers OS notifications when prefs allow.
type Sender struct {
	NS    *notifications.NotificationService
	Store *settings.Store

	mu       sync.Mutex
	lastSent time.Time
	// minGap avoids spamming when many refreshes complete close together.
	minGap time.Duration
}

// New constructs a Sender. ns or store may be nil (then sends are no-ops).
func New(ns *notifications.NotificationService, store *settings.Store) *Sender {
	return &Sender{
		NS:     ns,
		Store:  store,
		minGap: 3 * time.Second,
	}
}

// AfterRefresh notifies when articlesAdded > 0 and user enabled notifications.
func (s *Sender) AfterRefresh(ctx context.Context, articlesAdded int) {
	if s == nil || s.NS == nil || s.Store == nil || articlesAdded <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	prefs, err := s.Store.LoadUIPrefs(ctx)
	if err != nil {
		log.Printf("notify: load prefs: %v", err)
		return
	}
	if !prefs.NotifyOnNewArticles {
		return
	}

	s.mu.Lock()
	if s.minGap > 0 && time.Since(s.lastSent) < s.minGap {
		s.mu.Unlock()
		return
	}
	s.lastSent = time.Now()
	s.mu.Unlock()

	title := "LRSS"
	body := fmt.Sprintf("发现 %d 篇新文章", articlesAdded)
	if articlesAdded == 1 {
		body = "发现 1 篇新文章"
	}

	opts := notifications.NotificationOptions{
		ID:    fmt.Sprintf("lrss-new-%d", time.Now().UnixNano()),
		Title: title,
		Body:  body,
		Data: map[string]interface{}{
			"kind":  "new_articles",
			"added": articlesAdded,
		},
	}
	if !prefs.NotifySound {
		opts.Sound = &notifications.NotificationSound{Silent: true}
	}

	if err := s.NS.SendNotification(opts); err != nil {
		log.Printf("notify: send failed: %v", err)
	}
}

// Test sends a sample notification (for Settings UI). Respects sound preference.
func (s *Sender) Test(ctx context.Context) error {
	if s == nil || s.NS == nil {
		return fmt.Errorf("notifications unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sound := true
	if s.Store != nil {
		if prefs, err := s.Store.LoadUIPrefs(ctx); err == nil {
			sound = prefs.NotifySound
			// Test still works even if notifyOnNewArticles is off — user is testing the channel.
		}
	}
	opts := notifications.NotificationOptions{
		ID:    fmt.Sprintf("lrss-test-%d", time.Now().UnixNano()),
		Title: "LRSS",
		Body:  "这是一条测试通知",
		Data:  map[string]interface{}{"kind": "test"},
	}
	if !sound {
		opts.Sound = &notifications.NotificationSound{Silent: true}
	}
	return s.NS.SendNotification(opts)
}

// EnsureAuthorized requests OS permission when needed (macOS). Windows is always true.
func (s *Sender) EnsureAuthorized(ctx context.Context) (bool, error) {
	if s == nil || s.NS == nil {
		return false, fmt.Errorf("notifications unavailable")
	}
	ok, err := s.NS.CheckNotificationAuthorization()
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	return s.NS.RequestNotificationAuthorization()
}
