package htmltext

import (
	"strings"
	"testing"
)

func TestToText_Empty(t *testing.T) {
	if got := ToText(""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
	if got := ToText("   "); got != "" {
		t.Fatalf("whitespace: got %q", got)
	}
}

func TestToText_Plain(t *testing.T) {
	got := ToText("Hello world")
	if got != "Hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestToText_TagsAndEntities(t *testing.T) {
	in := `<p>Hello&nbsp;<b>world</b> &amp; friends</p>`
	got := ToText(in)
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "world") {
		t.Fatalf("missing words: %q", got)
	}
	wantSub := "friends"
	if !strings.Contains(got, wantSub) {
		t.Fatalf("got %q, want substring %q", got, wantSub)
	}
	if strings.Contains(got, "<b>") || strings.Contains(got, "</p>") {
		t.Fatalf("tags remain: %q", got)
	}
	if !strings.Contains(got, "&") {
		t.Fatalf("entity &amp; not decoded: %q", got)
	}
}

func TestToText_EnglishSample(t *testing.T) {
	in := `
		<html><body>
		<script>alert(1)</script>
		<style>.x{color:red}</style>
		<h1>Title</h1>
		<p>First paragraph.</p>
		<p>Second&nbsp;paragraph with <a href="http://ex.com">link</a>.</p>
		</body></html>
	`
	got := ToText(in)
	if strings.Contains(got, "alert") {
		t.Fatalf("script content leaked: %q", got)
	}
	if strings.Contains(got, "color") {
		t.Fatalf("style content leaked: %q", got)
	}
	for _, want := range []string{"Title", "First paragraph", "Second", "link"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("double space remains: %q", got)
	}
}

func TestToText_ChineseSample(t *testing.T) {
	in := `<div class="post"><p>你好，<strong>世界</strong>！</p><p>这是一篇&lt;测试&gt;文章。</p></div>`
	got := ToText(in)
	if !strings.Contains(got, "你好") {
		t.Fatalf("missing 你好: %q", got)
	}
	if !strings.Contains(got, "世界") {
		t.Fatalf("missing 世界: %q", got)
	}
	if !strings.Contains(got, "测试") {
		t.Fatalf("missing 测试: %q", got)
	}
	if strings.Contains(got, "<p>") || strings.Contains(got, "strong") {
		t.Fatalf("tags remain: %q", got)
	}
}

func TestToText_CollapseWhitespace(t *testing.T) {
	in := "<p>  a   \n\t  b  </p><p>c</p>"
	got := ToText(in)
	if strings.Contains(got, "  ") {
		t.Fatalf("not collapsed: %q", got)
	}
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") || !strings.Contains(got, "c") {
		t.Fatalf("missing content: %q", got)
	}
}
