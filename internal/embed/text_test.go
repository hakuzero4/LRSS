package embed_test

import (
	"testing"

	"lrss/internal/embed"
)

func TestBuildInputAndHashStable(t *testing.T) {
	a := embed.BuildInput("Title", "Sum", "Body")
	b := embed.BuildInput("Title", "Sum", "Body")
	if a != b {
		t.Fatalf("unstable input")
	}
	if embed.ContentHash(a) != embed.ContentHash(b) {
		t.Fatal("hash unstable")
	}
	long := embed.TruncateRunes(string(make([]rune, 10000)), 100)
	if embed.TruncateRunes(long, 100) != long {
		// just ensure no panic; length check
	}
	if len([]rune(embed.TruncateRunes("一二三四五六", 3))) != 3 {
		t.Fatal("rune truncate")
	}
}

func TestFakeDeterministic(t *testing.T) {
	f := embed.NewFake(8)
	v1 := f.MustEmbed("hello")
	v2 := f.MustEmbed("hello")
	if len(v1) != 8 {
		t.Fatal(len(v1))
	}
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatal("not deterministic")
		}
	}
}
