package repo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"lrss/internal/model"
	"lrss/internal/repo"
)

func seedKeepArticle(t *testing.T, r *repo.Repos, title string) string {
	t.Helper()
	ctx := context.Background()
	feed := &model.Feed{Title: "F-" + title, FeedURL: "https://ex.com/kf/" + title}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	body := "body"
	if _, err := r.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{GUID: "g-" + title, URL: "https://ex.com/" + title, Title: title, ContentText: &body, PublishedAt: &now},
	}); err != nil {
		t.Fatal(err)
	}
	arts, err := r.Articles.List(ctx, "all", repo.ListOpts{Limit: 20, Query: title})
	if err != nil || len(arts) == 0 {
		t.Fatalf("seed list: %v n=%d", err, len(arts))
	}
	return arts[0].ID
}

func TestKeepFolders_MoveListDeleteRoot(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	folderA, err := r.KeepFolders.Create(ctx, "Alpha", "")
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	folderB, err := r.KeepFolders.Create(ctx, "Beta", "")
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}

	articleID := seedKeepArticle(t, r, "KeepMe")
	if err := r.Articles.Keep(ctx, articleID, "pick", "manual", 1, []string{"news"}); err != nil {
		t.Fatalf("Keep: %v", err)
	}
	got, err := r.Articles.Get(ctx, articleID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepFolderID != "" {
		t.Fatalf("Keep should leave folder empty: %+v", got)
	}

	if err := r.Articles.SetKeepFolder(ctx, articleID, folderA.ID); err != nil {
		t.Fatalf("SetKeepFolder A: %v", err)
	}
	// Keep() must not overwrite folder_id on conflict.
	if err := r.Articles.Keep(ctx, articleID, "again", "manual", 1, nil); err != nil {
		t.Fatalf("Keep again: %v", err)
	}
	got, err = r.Articles.Get(ctx, articleID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepFolderID != folderA.ID {
		t.Fatalf("after SetKeepFolder A: %+v", got)
	}

	inA, err := r.Articles.List(ctx, "kept:"+folderA.ID, repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(inA) != 1 || inA[0].ID != articleID || inA[0].KeepFolderID != folderA.ID {
		t.Fatalf("List kept:A = %+v", inA)
	}
	inB, err := r.Articles.List(ctx, "kept:"+folderB.ID, repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(inB) != 0 {
		t.Fatalf("List kept:B = %+v want empty", inB)
	}
	allKept, err := r.Articles.List(ctx, "kept", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(allKept) != 1 || allKept[0].ID != articleID {
		t.Fatalf("List kept = %+v", allKept)
	}

	counts, err := r.Articles.CountKeptUnreadByFolder(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if counts[folderA.ID] != 1 || counts[""] != 0 {
		t.Fatalf("CountKeptUnreadByFolder = %v", counts)
	}
	listed, err := r.KeepFolders.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var unreadA, unreadB int
	for _, f := range listed {
		switch f.ID {
		case folderA.ID:
			unreadA = f.UnreadCount
		case folderB.ID:
			unreadB = f.UnreadCount
		}
	}
	if unreadA != 1 || unreadB != 0 {
		t.Fatalf("List unread A=%d B=%d", unreadA, unreadB)
	}

	if err := r.Articles.SetKeepFolder(ctx, articleID, ""); err != nil {
		t.Fatalf("SetKeepFolder root: %v", err)
	}
	got, err = r.Articles.Get(ctx, articleID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepFolderID != "" {
		t.Fatalf("after move to root: %+v", got)
	}
	inA, err = r.Articles.List(ctx, "kept:"+folderA.ID, repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(inA) != 0 {
		t.Fatalf("List kept:A after root = %+v", inA)
	}
	counts, err = r.Articles.CountKeptUnreadByFolder(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if counts[""] != 1 {
		t.Fatalf("root unread count = %v", counts)
	}

	if err := r.Articles.SetKeepFolder(ctx, articleID, folderA.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.KeepFolders.Delete(ctx, folderA.ID); err != nil {
		t.Fatalf("Delete A: %v", err)
	}
	if _, err := r.KeepFolders.Get(ctx, folderA.ID); err == nil {
		t.Fatal("deleted folder still present")
	}
	got, err = r.Articles.Get(ctx, articleID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepFolderID != "" {
		t.Fatalf("after delete folder: %+v", got)
	}
	allKept, err = r.Articles.List(ctx, "kept", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(allKept) != 1 || allKept[0].ID != articleID || allKept[0].KeepFolderID != "" {
		t.Fatalf("List kept after delete folder = %+v", allKept)
	}
}

func TestKeepFolders_CreateRules(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	t.Run("max depth 2", func(t *testing.T) {
		root, err := r.KeepFolders.Create(ctx, "Root", "")
		if err != nil {
			t.Fatalf("Create root: %v", err)
		}
		child, err := r.KeepFolders.Create(ctx, "Child", root.ID)
		if err != nil {
			t.Fatalf("Create child: %v", err)
		}
		if child.ParentID == nil || *child.ParentID != root.ID {
			t.Fatalf("child parent = %+v", child.ParentID)
		}
		if _, err := r.KeepFolders.Create(ctx, "Grand", child.ID); err == nil {
			t.Fatal("expected grandchild depth error")
		}
	})

	t.Run("duplicate name same parent", func(t *testing.T) {
		if _, err := r.KeepFolders.Create(ctx, "DupRoot", ""); err != nil {
			t.Fatalf("first: %v", err)
		}
		if _, err := r.KeepFolders.Create(ctx, "duproot", ""); err == nil {
			t.Fatal("expected duplicate root name error")
		}
		parent, err := r.KeepFolders.Create(ctx, "ParentDup", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.KeepFolders.Create(ctx, "Kid", parent.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := r.KeepFolders.Create(ctx, "kid", parent.ID); err == nil {
			t.Fatal("expected duplicate child name error")
		}
		other, err := r.KeepFolders.Create(ctx, "OtherParent", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := r.KeepFolders.Create(ctx, "Kid", other.ID); err != nil {
			t.Fatalf("same name under other parent should work: %v", err)
		}
	})

	t.Run("name required and max 80", func(t *testing.T) {
		if _, err := r.KeepFolders.Create(ctx, "  ", ""); err == nil {
			t.Fatal("expected empty name error")
		}
		if _, err := r.KeepFolders.Create(ctx, strings.Repeat("a", 81), ""); err == nil {
			t.Fatal("expected too-long name error")
		}
		if _, err := r.KeepFolders.Create(ctx, strings.Repeat("a", 80), ""); err != nil {
			t.Fatalf("80-rune name should work: %v", err)
		}
	})
}

func TestSetKeepFolder_Errors(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	folder, err := r.KeepFolders.Create(ctx, "Box", "")
	if err != nil {
		t.Fatal(err)
	}
	articleID := seedKeepArticle(t, r, "NotYetKept")
	if err := r.Articles.SetKeepFolder(ctx, articleID, folder.ID); err == nil {
		t.Fatal("SetKeepFolder on non-kept article should fail")
	}
	if err := r.Articles.Keep(ctx, articleID, "x", "manual", 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.Articles.SetKeepFolder(ctx, articleID, "missing-folder"); err == nil {
		t.Fatal("SetKeepFolder missing folder should fail")
	}
	if err := r.Articles.SetKeepFolder(ctx, "", folder.ID); err == nil {
		t.Fatal("SetKeepFolder empty article should fail")
	}
}

func TestKeepFolders_RenameGet(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	a, err := r.KeepFolders.Create(ctx, "One", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.KeepFolders.Create(ctx, "Two", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.KeepFolders.Rename(ctx, a.ID, "two"); err == nil {
		t.Fatal("rename to sibling name should fail")
	}
	if err := r.KeepFolders.Rename(ctx, a.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	got, err := r.KeepFolders.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed" {
		t.Fatalf("name = %q", got.Name)
	}
}
