package repo_test

import (
	"testing"

	"lrss/internal/model"
)

func TestChat_GetOrCreateAndMessages(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := t.Context()

	s1, err := r.Chats.GetOrCreateByArticle(ctx, "art-1", "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID == "" || s1.ArticleID != "art-1" {
		t.Fatalf("session = %+v", s1)
	}
	s2, err := r.Chats.GetOrCreateByArticle(ctx, "art-1", "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if s2.ID != s1.ID {
		t.Fatalf("expected same session, %s vs %s", s1.ID, s2.ID)
	}

	u := &model.ChatMessage{SessionID: s1.ID, Role: "user", Content: "这篇在说什么？"}
	if err := r.Chats.InsertMessage(ctx, u); err != nil {
		t.Fatal(err)
	}
	a := &model.ChatMessage{
		SessionID: s1.ID,
		Role:      "assistant",
		Content:   "它在讲 Widget。[0]",
		Citations: []model.ChatCitation{{N: 0, ArticleID: "art-1", Title: "Widget"}},
	}
	if err := r.Chats.InsertMessage(ctx, a); err != nil {
		t.Fatal(err)
	}
	msgs, err := r.Chats.ListMessages(ctx, s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Citations[0].ArticleID != "art-1" {
		t.Fatalf("msgs = %#v", msgs)
	}

	if err := r.Chats.DeleteByArticle(ctx, "art-1"); err != nil {
		t.Fatal(err)
	}
	msgs, err = r.Chats.ListMessages(ctx, s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected cascade delete, got %d", len(msgs))
	}
}

func TestChat_GetOrCreateRequiresArticle(t *testing.T) {
	r, _ := openTestRepos(t, false)
	if _, err := r.Chats.GetOrCreateByArticle(t.Context(), "  ", ""); err == nil {
		t.Fatal("expected error")
	}
}
