package llm_test

import (
	"strings"
	"testing"

	"lrss/internal/llm"
)

func TestParseBilingualPairs_Markers(t *testing.T) {
	raw := `
<<o>> As long as you're alive, there's no bad ending.
<<t>> 只要你还活着，就没有真正的结局。

<<o>> I believe comedians never truly retire.
<<t>> 我相信喜剧演员永远不会真正退休。
`
	pairs := llm.ParseBilingualPairs(raw)
	if len(pairs) != 2 {
		t.Fatalf("len=%d %+v", len(pairs), pairs)
	}
	if !strings.Contains(pairs[0].Original, "alive") {
		t.Fatalf("orig0=%q", pairs[0].Original)
	}
	if !strings.Contains(pairs[0].Translation, "活着") {
		t.Fatalf("tgt0=%q", pairs[0].Translation)
	}
	if !strings.Contains(pairs[1].Translation, "喜剧") {
		t.Fatalf("tgt1=%q", pairs[1].Translation)
	}
}

func TestParseBilingualPairs_FallbackBlocks(t *testing.T) {
	raw := "Hello world.\n你好世界。\n\nSecond line.\n第二行。"
	pairs := llm.ParseBilingualPairs(raw)
	if len(pairs) < 2 {
		t.Fatalf("pairs=%+v", pairs)
	}
}

func TestUserPromptTranslate_HasMarkers(t *testing.T) {
	p := llm.UserPromptTranslate("Title: x\nBody:\nhi", "zh-CN")
	if !strings.Contains(p, "<<o>>") || !strings.Contains(p, "<<t>>") {
		t.Fatalf("prompt missing markers: %s", p)
	}
	if !strings.Contains(p, "简体中文") {
		t.Fatal("expected zh label")
	}
}

func TestParseFullnessVerdict(t *testing.T) {
	if llm.ParseFullnessVerdict("VERDICT: partial\ncut off") != llm.FullnessPartial {
		t.Fatal("partial")
	}
	if llm.ParseFullnessVerdict("VERDICT: full") != llm.FullnessFull {
		t.Fatal("full")
	}
	if llm.ParseFullnessVerdict("maybe?") != llm.FullnessUnclear {
		t.Fatal("unclear")
	}
	p := llm.UserPromptContentFullness("T", "S", "Body…", "https://ex.com")
	if !strings.Contains(p, "VERDICT:") || !strings.Contains(p, "Body…") {
		t.Fatalf("prompt = %s", p)
	}
}

func TestUserPromptSelectTranslate_PlainOnly(t *testing.T) {
	p := llm.UserPromptSelectTranslate("hello world", "zh-CN")
	if strings.Contains(p, "<<o>>") || strings.Contains(p, "<<t>>") {
		t.Fatalf("select translate must not use bilingual markers: %s", p)
	}
	if !strings.Contains(p, "hello world") {
		t.Fatal("expected source text in prompt")
	}
	if !strings.Contains(p, "ONLY the translation") {
		t.Fatal("expected output-only instruction")
	}
	sys := llm.SystemPromptFor(llm.FeatureSelectTranslate, "zh-CN")
	if strings.TrimSpace(sys) == "" {
		t.Fatal("empty system prompt")
	}
}

func TestTranslatedBodyFromPairs(t *testing.T) {
	html, plain := llm.TranslatedBodyFromPairs([]llm.BilingualPair{
		{Original: "Hello.", Translation: "你好。"},
		{Original: "World.", Translation: "世界。"},
	})
	if !strings.Contains(html, "<p>你好。</p>") || !strings.Contains(html, "世界") {
		t.Fatalf("html=%q", html)
	}
	if plain != "你好。\n世界。" {
		t.Fatalf("plain=%q", plain)
	}
}
