package llm_test

import (
	"context"
	"strings"
	"testing"

	"lrss/internal/llm"
	"lrss/internal/settings"
)

func TestNormalizeKeepStrictness(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"loose", llm.StrictnessLoose},
		{"Loose", llm.StrictnessLoose},
		{"  LOOSE ", llm.StrictnessLoose},
		{"宽松", llm.StrictnessLoose},
		{"strict", llm.StrictnessStrict},
		{"STRICT", llm.StrictnessStrict},
		{"严格", llm.StrictnessStrict},
		{"", llm.StrictnessStandard},
		{"standard", llm.StrictnessStandard},
		{"标准", llm.StrictnessStandard},
		{"nope", llm.StrictnessStandard},
	}
	for _, tc := range cases {
		if got := llm.NormalizeKeepStrictness(tc.in); got != tc.want {
			t.Errorf("NormalizeKeepStrictness(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestKeepConfidenceThreshold(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"loose", 0.55},
		{"宽松", 0.55},
		{"strict", 0.85},
		{"严格", 0.85},
		{"", 0.70},
		{"standard", 0.70},
		{"other", 0.70},
	}
	for _, tc := range cases {
		if got := llm.KeepConfidenceThreshold(tc.in); got != tc.want {
			t.Errorf("KeepConfidenceThreshold(%q) = %v want %v", tc.in, got, tc.want)
		}
	}
}

func TestUserPromptKeep_ContainsProfileThresholdLocale(t *testing.T) {
	profile := "systems programming; skip celebrity gossip"
	p := llm.UserPromptKeep([]llm.KeepItem{
		{Title: "Go 1.23", Feed: "Golang Weekly", Published: "2024-08-01", Summary: "release notes", Body: "the toolchain ships"},
	}, profile, "standard", "zh-CN", nil)
	for _, want := range []string{
		profile,
		"0.70",
		"[1]",
		"简体中文",
		"Title: Go 1.23",
		"Feed: Golang Weekly",
		"EMPTY PROFILE",
	} {
		if want == "EMPTY PROFILE" {
			if strings.Contains(p, want) {
				t.Fatalf("non-empty profile should not say EMPTY PROFILE:\n%s", p)
			}
			continue
		}
		if !strings.Contains(p, want) {
			t.Fatalf("user prompt missing %q:\n%s", want, p)
		}
	}
	empty := llm.UserPromptKeep(nil, "  ", "loose", "en-US", nil)
	if !strings.Contains(empty, "EMPTY PROFILE") {
		t.Fatalf("empty profile: %s", empty)
	}
	if !strings.Contains(empty, "0.55") {
		t.Fatalf("loose threshold missing: %s", empty)
	}
	if !strings.Contains(empty, "Write the entire reply in English") {
		t.Fatalf("en locale instruction missing: %s", empty)
	}
	if strings.Contains(empty, "Subfolders") || strings.Contains(empty, `"folder"`) {
		t.Fatalf("empty folders should not mention routing:\n%s", empty)
	}
}

func TestUserPromptKeep_ListsFoldersAndRequiresFolderField(t *testing.T) {
	p := llm.UserPromptKeep([]llm.KeepItem{{Title: "T"}}, "profile", "standard", "en", []llm.KeepFolderRef{
		{ID: "r", Name: "Rust", Hint: "systems language"},
		{ID: "g", Name: "Go"},
		{ID: "x", Name: "  "},
	})
	for _, want := range []string{
		"- Rust — systems language",
		"- Go",
		`"folder":"Name or empty"`,
		"NEVER invent a folder name",
		"keep=false → omit folder",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("folder prompt missing %q:\n%s", want, p)
		}
	}
}

func TestSystemPromptFor_FeatureKeep(t *testing.T) {
	p := llm.SystemPromptFor(llm.FeatureKeep, "en-US")
	if strings.TrimSpace(p) == "" {
		t.Fatal("empty keep system prompt")
	}
	for _, want := range []string{
		"senior RSS desk editor",
		"worth reading",
		"ONLY valid JSON",
		"When subfolders are listed",
		"Never invent folders",
		"Write the entire reply in English",
	} {
		if !strings.Contains(p, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, p)
		}
	}
}

func TestParseKeepBatch_FencedTrailingUnknownStringConf(t *testing.T) {
	byIndex := map[int]llm.KeepItem{
		1: {ID: "a1", Title: "A"},
		2: {ID: "a2", Title: "B"},
	}
	raw := "```json\n" + `{
		"items": [
			{"n": 1, "keep": true, "confidence": "0.82", "reason": "new API shipped", "topics": ["go"],},
			{"n": 99, "keep": true, "confidence": 0.99, "reason": "ghost", "topics": "x"},
			{"n": 2, "keep": true, "confidence": 0.4, "reason": "ok", "topics": ["", "rss", " "]},
		],
	}` + "\n```"
	got := llm.ParseKeepBatch(raw, byIndex)
	if len(got) != 2 {
		t.Fatalf("len = %d want 2 (unknown n dropped): %+v", len(got), got)
	}
	if got[0].ArticleID != "a1" || !got[0].Keep || got[0].Confidence != 0.82 {
		t.Fatalf("item1 = %+v", got[0])
	}
	if got[0].Reason != "new API shipped" || len(got[0].Topics) != 1 || got[0].Topics[0] != "go" {
		t.Fatalf("item1 meta = %+v", got[0])
	}
	if got[1].ArticleID != "a2" || !got[1].Keep || got[1].Confidence != 0.4 {
		t.Fatalf("item2 = %+v (raw keep must survive parse)", got[1])
	}
	if len(got[1].Topics) != 1 || got[1].Topics[0] != "rss" {
		t.Fatalf("item2 topics = %v", got[1].Topics)
	}
}

func TestParseKeepBatch_InvalidAndMissing(t *testing.T) {
	byIndex := map[int]llm.KeepItem{1: {ID: "a1"}}
	if got := llm.ParseKeepBatch("not json at all", byIndex); len(got) != 0 {
		t.Fatalf("invalid json: %+v", got)
	}
	if got := llm.ParseKeepBatch(`{"overview":[]}`, byIndex); len(got) != 0 {
		t.Fatalf("no items: %+v", got)
	}
	got := llm.ParseKeepBatch(`{"items":[{"n":1,"confidence":0.9}]}`, byIndex)
	if len(got) != 1 || got[0].Keep || got[0].Confidence != 0.9 {
		t.Fatalf("missing keep: %+v", got)
	}
	kept := llm.ParseKeepBatch(`{"items":[{"n":1,"keep":true,"confidence":0.8}]}`, byIndex)
	if len(kept) != 1 || !kept[0].Keep || kept[0].Reason != "worth reading" {
		t.Fatalf("empty reason fallback: %+v", kept)
	}
	single := llm.ParseKeepBatch(`{"items":{"n":1,"keep":true,"confidence":"1.5","topics":"ml"}}`, byIndex)
	if len(single) != 1 || single[0].Confidence != 1 || len(single[0].Topics) != 1 || single[0].Topics[0] != "ml" {
		t.Fatalf("single item + clamp + topic string: %+v", single)
	}
}

func TestJudgeKeepBatch_ThresholdAndOrder(t *testing.T) {
	store, _ := testStore(t)
	stub := &stubChat{
		model: "test-model",
		content: `{"items":[
			{"n":2,"keep":true,"confidence":0.91,"reason":"solid report","topics":["ai"]},
			{"n":1,"keep":true,"confidence":0.40,"reason":"thin teaser","topics":["x"]}
		]}`,
	}
	svc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	items := []llm.KeepItem{
		{Index: 1, ID: "id-a", Title: "A", Feed: "F", Summary: "sa", Body: "ba"},
		{Index: 2, ID: "id-b", Title: "B", Feed: "F", Summary: "sb", Body: "bb"},
	}
	got, err := svc.JudgeKeepBatch(context.Background(), items, "golang", "standard", "en-US", nil)
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 {
		t.Fatalf("calls = %d", stub.calls)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d: %+v", len(got), got)
	}
	if got[0].ArticleID != "id-a" || got[0].Keep || got[0].Confidence != 0.40 {
		t.Fatalf("first should flip below 0.70: %+v", got[0])
	}
	if got[1].ArticleID != "id-b" || !got[1].Keep || got[1].Confidence != 0.91 {
		t.Fatalf("second should stay keep: %+v", got[1])
	}
	if !strings.Contains(stub.lastSys, "senior RSS desk editor") {
		t.Fatalf("system prompt: %s", stub.lastSys)
	}
	if !strings.Contains(stub.lastUser, "golang") || !strings.Contains(stub.lastUser, "0.70") {
		t.Fatalf("user prompt: %s", stub.lastUser)
	}
}

func TestJudgeKeepBatch_EmptyAndMissing(t *testing.T) {
	svc := &llm.Service{}
	got, err := svc.JudgeKeepBatch(context.Background(), nil, "", "", "", nil)
	if err != nil || got != nil {
		t.Fatalf("empty items: got=%v err=%v", got, err)
	}

	store, _ := testStore(t)
	stub := &stubChat{model: "test-model", content: `{"items":[{"n":1,"keep":true,"confidence":0.99,"reason":"only one"}]}`}
	svc = &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	items := []llm.KeepItem{
		{ID: "keep-me", Title: "A"},
		{ID: "miss-me", Title: "B"},
	}
	got, err = svc.JudgeKeepBatch(context.Background(), items, "", "loose", "en", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Keep || got[1].Keep || got[1].Confidence != 0 || got[1].Reason != "" {
		t.Fatalf("missing model item should be keep=false: %+v", got)
	}
}

func TestMatchKeepFolder(t *testing.T) {
	folders := []llm.KeepFolderRef{
		{ID: "id-rust", Name: "Rust", Hint: "systems"},
		{ID: "id-go", Name: "Go"},
	}
	id, name := llm.MatchKeepFolder("Rust", folders)
	if id != "id-rust" || name != "Rust" {
		t.Fatalf("exact: id=%q name=%q", id, name)
	}
	id, name = llm.MatchKeepFolder("  rust  ", folders)
	if id != "id-rust" || name != "Rust" {
		t.Fatalf("case-insensitive: id=%q name=%q", id, name)
	}
	id, name = llm.MatchKeepFolder("NoSuch", folders)
	if id != "" || name != "" {
		t.Fatalf("miss: id=%q name=%q", id, name)
	}
	id, name = llm.MatchKeepFolder("  ", folders)
	if id != "" || name != "" {
		t.Fatalf("empty: id=%q name=%q", id, name)
	}
	id, name = llm.MatchKeepFolder("Rus", folders)
	if id != "" || name != "" {
		t.Fatalf("substring must not match: id=%q name=%q", id, name)
	}
	id, name = llm.MatchKeepFolder("Go", []llm.KeepFolderRef{
		{ID: "first", Name: "Go"},
		{ID: "second", Name: "Go"},
	})
	if id != "first" || name != "Go" {
		t.Fatalf("first match wins: id=%q name=%q", id, name)
	}
}

func TestParseKeepBatch_FolderString(t *testing.T) {
	byIndex := map[int]llm.KeepItem{
		1: {ID: "a1"},
		2: {ID: "a2"},
		3: {ID: "a3"},
	}
	got := llm.ParseKeepBatch(`{"items":[
		{"n":1,"keep":true,"confidence":0.9,"reason":"ok","folder":"Rust"},
		{"n":2,"keep":true,"confidence":0.8,"reason":"ok","folderName":"Go"},
		{"n":3,"keep":false,"confidence":0.2,"reason":"no","folder":"Rust"}
	]}`, byIndex)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(got), got)
	}
	if got[0].FolderName != "Rust" || got[0].FolderID != "" {
		t.Fatalf("folder stored raw, id unresolved: %+v", got[0])
	}
	if got[1].FolderName != "Go" || got[1].FolderID != "" {
		t.Fatalf("folderName alias: %+v", got[1])
	}
	if got[2].FolderName != "Rust" || got[2].Keep {
		t.Fatalf("parse keeps raw folder even when keep=false: %+v", got[2])
	}
}

func TestJudgeKeepBatch_FolderRouting(t *testing.T) {
	store, _ := testStore(t)
	items := []llm.KeepItem{{ID: "art-1", Title: "T", Body: "body"}}
	folders := []llm.KeepFolderRef{{ID: "fid-rust", Name: "Rust", Hint: "lang"}}

	t.Run("no folders keep true clears folder", func(t *testing.T) {
		stub := &stubChat{model: "test-model", content: `{"items":[{"n":1,"keep":true,"confidence":0.9,"reason":"ok","folder":"Rust"}]}`}
		svc := &llm.Service{
			Store: store,
			NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
				return stub, nil
			},
		}
		got, err := svc.JudgeKeepBatch(context.Background(), items, "", "loose", "en", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !got[0].Keep || got[0].FolderID != "" || got[0].FolderName != "" {
			t.Fatalf("root when no folders: %+v", got)
		}
		if strings.Contains(stub.lastUser, "Subfolders") {
			t.Fatalf("user prompt should not mention routing: %s", stub.lastUser)
		}
	})

	t.Run("case-insensitive folder match", func(t *testing.T) {
		stub := &stubChat{model: "test-model", content: `{"items":[{"n":1,"keep":true,"confidence":0.9,"reason":"ok","folder":"rust"}]}`}
		svc := &llm.Service{
			Store: store,
			NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
				return stub, nil
			},
		}
		got, err := svc.JudgeKeepBatch(context.Background(), items, "", "loose", "en", folders)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !got[0].Keep || got[0].FolderID != "fid-rust" || got[0].FolderName != "Rust" {
			t.Fatalf("want Rust id: %+v", got)
		}
		if !strings.Contains(stub.lastUser, "- Rust — lang") {
			t.Fatalf("user prompt missing folder list: %s", stub.lastUser)
		}
	})

	t.Run("unknown folder name is root but keep stays", func(t *testing.T) {
		stub := &stubChat{model: "test-model", content: `{"items":[{"n":1,"keep":true,"confidence":0.9,"reason":"ok","folder":"NoSuch"}]}`}
		svc := &llm.Service{
			Store: store,
			NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
				return stub, nil
			},
		}
		got, err := svc.JudgeKeepBatch(context.Background(), items, "", "loose", "en", folders)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !got[0].Keep || got[0].FolderID != "" || got[0].FolderName != "" {
			t.Fatalf("unknown name → root, still keep: %+v", got)
		}
	})

	t.Run("keep false clears folder", func(t *testing.T) {
		stub := &stubChat{model: "test-model", content: `{"items":[{"n":1,"keep":false,"confidence":0.95,"reason":"promo","folder":"Rust"}]}`}
		svc := &llm.Service{
			Store: store,
			NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
				return stub, nil
			},
		}
		got, err := svc.JudgeKeepBatch(context.Background(), items, "", "loose", "en", folders)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Keep || got[0].FolderID != "" || got[0].FolderName != "" {
			t.Fatalf("not kept, folder empty: %+v", got)
		}
	})
}
