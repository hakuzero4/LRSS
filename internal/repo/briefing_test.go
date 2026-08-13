package repo_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"lrss/internal/model"
)

func sampleBriefingPayload() model.BriefingPayload {
	return model.BriefingPayload{
		Overview: "Daily wrap of three feeds.",
		Themes: []model.BriefingTheme{
			{
				Title: "AI tooling",
				Bullets: []model.BriefingBullet{
					{ArticleID: "a1", Title: "New model", FeedTitle: "Tech", Point: "A smaller model shipped."},
				},
			},
		},
		Watch: []model.BriefingBullet{
			{ArticleID: "a2", Title: "Outage", FeedTitle: "Ops", Point: "Status page still yellow."},
		},
	}
}

func TestBriefing_InsertListGetPayload(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	payload := sampleBriefingPayload()
	tests := []struct {
		name string
		in   *model.Briefing
	}{
		{
			name: "full payload",
			in: &model.Briefing{
				Status:       "ready",
				Locale:       "zh-CN",
				Model:        "gpt-test",
				Overview:     payload.Overview,
				ArticleCount: 3,
				OmittedCount: 1,
				Payload:      payload,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := r.Briefings.Insert(ctx, tt.in); err != nil {
				t.Fatalf("Insert: %v", err)
			}
			if tt.in.ID == "" {
				t.Fatal("expected generated id")
			}
			if tt.in.CreatedAt == "" {
				t.Fatal("expected generated created_at")
			}

			got, err := r.Briefings.Get(ctx, tt.in.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.ID != tt.in.ID || got.Status != tt.in.Status || got.Locale != tt.in.Locale {
				t.Fatalf("Get meta = %+v", got)
			}
			if got.Model != tt.in.Model || got.Overview != tt.in.Overview {
				t.Fatalf("Get model/overview = %q %q", got.Model, got.Overview)
			}
			if got.ArticleCount != tt.in.ArticleCount || got.OmittedCount != tt.in.OmittedCount {
				t.Fatalf("Get counts = %d/%d", got.ArticleCount, got.OmittedCount)
			}
			if !reflect.DeepEqual(got.Payload, tt.in.Payload) {
				t.Fatalf("Get payload = %#v want %#v", got.Payload, tt.in.Payload)
			}

			list, err := r.Briefings.List(ctx, 0)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(list) != 1 {
				t.Fatalf("List len = %d want 1", len(list))
			}
			if !reflect.DeepEqual(list[0].Payload, tt.in.Payload) {
				t.Fatalf("List payload = %#v want %#v", list[0].Payload, tt.in.Payload)
			}
			if list[0].ID != tt.in.ID {
				t.Fatalf("List id = %s want %s", list[0].ID, tt.in.ID)
			}
		})
	}
}

func TestBriefing_SetReadUnreadCount(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	first := &model.Briefing{Status: "ready", Locale: "en-US", Overview: "one"}
	second := &model.Briefing{Status: "ready", Locale: "en-US", Overview: "two"}
	if err := r.Briefings.Insert(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := r.Briefings.Insert(ctx, second); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		mutate  func() error
		wantN   int
		wantErr bool
	}{
		{
			name:  "both unread after insert",
			wantN: 2,
		},
		{
			name:   "mark first read",
			mutate: func() error { return r.Briefings.SetRead(ctx, first.ID, true) },
			wantN:  1,
		},
		{
			name:   "mark first unread again",
			mutate: func() error { return r.Briefings.SetRead(ctx, first.ID, false) },
			wantN:  2,
		},
		{
			name:    "missing id",
			mutate:  func() error { return r.Briefings.SetRead(ctx, "missing", true) },
			wantErr: true,
			wantN:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mutate != nil {
				err := tt.mutate()
				if tt.wantErr {
					if err == nil {
						t.Fatal("expected error")
					}
				} else if err != nil {
					t.Fatalf("mutate: %v", err)
				}
			}
			n, err := r.Briefings.UnreadCount(ctx)
			if err != nil {
				t.Fatalf("UnreadCount: %v", err)
			}
			if n != tt.wantN {
				t.Fatalf("UnreadCount = %d want %d", n, tt.wantN)
			}
		})
	}
}

func TestBriefing_Delete(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()
	b := &model.Briefing{Status: "ready", Locale: "zh-CN", Overview: "gone"}
	if err := r.Briefings.Insert(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := r.Briefings.Delete(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Briefings.Get(ctx, b.ID); err == nil {
		t.Fatal("expected get after delete to fail")
	}
	if err := r.Briefings.Delete(ctx, b.ID); err == nil {
		t.Fatal("expected second delete to fail")
	}
}

func TestBriefing_SetStarredPruneOld(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	starredIDs := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		b := &model.Briefing{
			CreatedAt: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Status:    "ready",
			Locale:    "zh-CN",
			Overview:  "starred",
		}
		if err := r.Briefings.Insert(ctx, b); err != nil {
			t.Fatalf("insert starred seed %d: %v", i, err)
		}
		if err := r.Briefings.SetStarred(ctx, b.ID, true); err != nil {
			t.Fatalf("SetStarred: %v", err)
		}
		starredIDs = append(starredIDs, b.ID)
	}
	for i := 0; i < 35; i++ {
		b := &model.Briefing{
			CreatedAt: base.Add(time.Duration(10+i) * time.Second).Format(time.RFC3339),
			Status:    "ready",
			Locale:    "en-US",
			Overview:  "plain",
		}
		if err := r.Briefings.Insert(ctx, b); err != nil {
			t.Fatalf("insert unstarred %d: %v", i, err)
		}
	}

	deleted, err := r.Briefings.PruneOld(ctx, 30)
	if err != nil {
		t.Fatalf("PruneOld: %v", err)
	}
	if deleted != 5 {
		t.Fatalf("PruneOld deleted = %d want 5", deleted)
	}

	list, err := r.Briefings.List(ctx, 100)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var starred, unstarred int
	remainStar := map[string]bool{}
	for _, b := range list {
		if b.IsStarred {
			starred++
			remainStar[b.ID] = true
		} else {
			unstarred++
		}
	}
	if starred != 2 {
		t.Fatalf("starred remaining = %d want 2", starred)
	}
	if unstarred != 30 {
		t.Fatalf("unstarred remaining = %d want 30", unstarred)
	}
	for _, id := range starredIDs {
		if !remainStar[id] {
			t.Fatalf("starred %s was pruned", id)
		}
		if _, err := r.Briefings.Get(ctx, id); err != nil {
			t.Fatalf("Get starred %s: %v", id, err)
		}
	}

	deleted2, err := r.Briefings.PruneOld(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if deleted2 != 0 {
		t.Fatalf("second PruneOld deleted = %d want 0", deleted2)
	}
}

func TestBriefing_UpdateGeneratedPendingToReady(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	pending := &model.Briefing{
		Status: "pending",
		Locale: "zh-CN",
	}
	if err := r.Briefings.Insert(ctx, pending); err != nil {
		t.Fatal(err)
	}

	payload := sampleBriefingPayload()
	tests := []struct {
		name         string
		id           string
		status       string
		modelName    string
		overview     string
		errMsg       string
		articleCount int
		omittedCount int
		payload      model.BriefingPayload
		wantErr      bool
	}{
		{
			name:         "pending to ready",
			id:           pending.ID,
			status:       "ready",
			modelName:    "local-qwen",
			overview:     payload.Overview,
			articleCount: 8,
			omittedCount: 2,
			payload:      payload,
		},
		{
			name:    "missing",
			id:      "no-such",
			status:  "ready",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Briefings.UpdateGenerated(ctx, tt.id, tt.status, tt.modelName, tt.overview, tt.errMsg, tt.articleCount, tt.omittedCount, tt.payload)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateGenerated: %v", err)
			}
			got, err := r.Briefings.Get(ctx, tt.id)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Status != tt.status {
				t.Fatalf("status = %q want %q", got.Status, tt.status)
			}
			if got.Model != tt.modelName {
				t.Fatalf("model = %q want %q", got.Model, tt.modelName)
			}
			if got.Overview != tt.overview {
				t.Fatalf("overview = %q want %q", got.Overview, tt.overview)
			}
			if got.Error != tt.errMsg {
				t.Fatalf("error = %q want %q", got.Error, tt.errMsg)
			}
			if got.ArticleCount != tt.articleCount || got.OmittedCount != tt.omittedCount {
				t.Fatalf("counts = %d/%d want %d/%d", got.ArticleCount, got.OmittedCount, tt.articleCount, tt.omittedCount)
			}
			if !reflect.DeepEqual(got.Payload, tt.payload) {
				t.Fatalf("payload = %#v want %#v", got.Payload, tt.payload)
			}
			if got.Locale != "zh-CN" {
				t.Fatalf("locale should be unchanged, got %q", got.Locale)
			}
		})
	}
}

func TestBriefing_ListOrderAndDefaultLimit(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// Same created_at: newer id must sort first (ORDER BY created_at DESC, id DESC).
	same := base.Format(time.RFC3339)
	older := &model.Briefing{CreatedAt: base.Add(-time.Hour).Format(time.RFC3339), Status: "ready", Locale: "en-US", Overview: "old"}
	a := &model.Briefing{CreatedAt: same, Status: "ready", Locale: "en-US", Overview: "a"}
	b := &model.Briefing{CreatedAt: same, Status: "ready", Locale: "en-US", Overview: "b"}
	if err := r.Briefings.Insert(ctx, older); err != nil {
		t.Fatal(err)
	}
	if err := r.Briefings.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := r.Briefings.Insert(ctx, b); err != nil {
		t.Fatal(err)
	}

	list, err := r.Briefings.List(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d want 2", len(list))
	}
	// a then b inserted with same timestamp; ULID id of b is larger.
	if list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("order = %s,%s want %s,%s", list[0].ID, list[1].ID, b.ID, a.ID)
	}
}
