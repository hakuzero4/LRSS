package llm_test

import (
	"strings"
	"testing"

	"lrss/internal/llm"
)

func TestSystemPromptBriefing_IsNewsDeskNotGenericSummary(t *testing.T) {
	p := llm.SystemPromptFor(llm.FeatureBriefing, "zh-CN")
	if !strings.Contains(p, "what is actually valuable") {
		t.Fatalf("missing value question: %s", p)
	}
	if !strings.Contains(p, "Restating or lightly paraphrasing titles") {
		t.Fatalf("missing title ban: %s", p)
	}
	if !strings.Contains(p, "Output ONLY valid JSON") {
		t.Fatalf("missing JSON rule: %s", p)
	}
	if strings.Contains(strings.ToLower(p), "summarize these articles") {
		t.Fatal("must not use generic summarize-these-articles prompt")
	}
}

func TestParseAndMapBriefing_ValidAndDropsUnknownN(t *testing.T) {
	raw := `{
		"overview": "OpenAI shipped a model. <b>x</b>",
		"themes": [
			{"title": "Models", "bullets": [
				{"n": [1, 99], "point": "GPT-5 announced"},
				{"n": [0], "point": "should drop"}
			]},
			{"title": "Empty", "bullets": [{"n": [8], "point": "unknown only"}]}
		],
		"watch": [{"n": [2], "point": "Hearing on Friday"}]
	}`
	got, err := llm.ParseAndMapBriefing(raw, map[int]llm.BriefingSource{
		1: {ID: "id-a", Title: "A", FeedTitle: "FeedA"},
		2: {ID: "id-b", Title: "B", FeedTitle: "FeedB"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Overview, "OpenAI") || strings.Contains(got.Overview, "<b>") {
		t.Fatalf("overview = %q", got.Overview)
	}
	if len(got.Themes) != 1 || got.Themes[0].Title != "Models" {
		t.Fatalf("themes = %+v", got.Themes)
	}
	if len(got.Themes[0].Bullets) != 1 || got.Themes[0].Bullets[0].ArticleID != "id-a" {
		t.Fatalf("bullets = %+v", got.Themes[0].Bullets)
	}
	if n := len(got.Themes[0].Bullets[0].Cites); n != 1 {
		t.Fatalf("cites = %d want 1 (unknown n dropped)", n)
	}
	if len(got.Watch) != 1 || got.Watch[0].ArticleID != "id-b" {
		t.Fatalf("watch = %+v", got.Watch)
	}
}

func TestParseAndMapBriefing_FenceAndEmptyThemes(t *testing.T) {
	_, err := llm.ParseAndMapBriefing("```json\n{\"overview\":\"x\",\"themes\":[]}\n```", nil)
	if err == nil {
		t.Fatal("expected empty themes error")
	}
	raw := "```json\n{\"overview\":\"shift\",\"themes\":[{\"title\":\"T\",\"bullets\":[{\"n\":[1],\"point\":\"fact\"}]}]}\n```"
	got, err := llm.ParseAndMapBriefing(raw, map[int]llm.BriefingSource{
		1: {ID: "x", Title: "T1", FeedTitle: "F"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Themes) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestParseAndMapBriefing_WatchAsStringArray(t *testing.T) {
	// qwen-local often emits watch as []string; that used to fail the whole briefing.
	raw := `{
		"overview": "shift",
		"themes": [{"title":"T","bullets":[{"n":[1],"point":"fact"}]}],
		"watch": ["周五听证会", "[2] 周五听证会"]
	}`
	got, err := llm.ParseAndMapBriefing(raw, map[int]llm.BriefingSource{
		1: {ID: "a", Title: "A", FeedTitle: "F"},
		2: {ID: "b", Title: "B", FeedTitle: "F"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Themes) != 1 {
		t.Fatalf("themes = %+v", got.Themes)
	}
	if len(got.Watch) != 1 || got.Watch[0].ArticleID != "b" {
		t.Fatalf("watch = %+v", got.Watch)
	}
}

func TestParseAndMapBriefing_WatchStringDoesNotFail(t *testing.T) {
	raw := `{"overview":"x","themes":[{"title":"T","bullets":[{"n":[1],"point":"fact"}]}],"watch":"just a note"}`
	got, err := llm.ParseAndMapBriefing(raw, map[int]llm.BriefingSource{
		1: {ID: "a", Title: "A", FeedTitle: "F"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Watch) != 0 {
		t.Fatalf("watch without cites should drop, got %+v", got.Watch)
	}
}

func TestParseAndMapBriefing_LooseNAndPreamble(t *testing.T) {
	raw := "here you go\n{\"overview\":\"x\",\"themes\":[{\"title\":\"T\",\"bullets\":[{\"n\":1,\"point\":\"fact\"}]}],\"watch\":{\"n\":\"2\",\"point\":\"soon\"}}\nthanks"
	got, err := llm.ParseAndMapBriefing(raw, map[int]llm.BriefingSource{
		1: {ID: "a", Title: "A", FeedTitle: "F"},
		2: {ID: "b", Title: "B", FeedTitle: "F"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Themes) != 1 || got.Themes[0].Bullets[0].ArticleID != "a" {
		t.Fatalf("themes = %+v", got.Themes)
	}
	if len(got.Watch) != 1 || got.Watch[0].ArticleID != "b" {
		t.Fatalf("watch = %+v", got.Watch)
	}
}

func TestParseAndMapBriefing_TrailingCommaAndStringBullets(t *testing.T) {
	raw := `{
		"overview": "x",
		"themes": [{"title":"T","bullets":["[1] shipped today", {"n":[1],"point":"also ok"}],}],
		"watch": [],
	}`
	got, err := llm.ParseAndMapBriefing(raw, map[int]llm.BriefingSource{
		1: {ID: "a", Title: "A", FeedTitle: "F"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got.Themes[0].Bullets); n != 2 {
		t.Fatalf("bullets = %d want 2: %+v", n, got.Themes[0].Bullets)
	}
}
