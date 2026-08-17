package llm

import "testing"

func TestApplyKeepThreshold_FlipsLowConfidence(t *testing.T) {
	items := []KeepItem{{ID: "a", Index: 1}, {ID: "b", Index: 2}, {ID: "c", Index: 3}}
	parsed := []KeepVerdict{
		{ArticleID: "a", Keep: true, Confidence: 0.90, Reason: "fit"},
		{ArticleID: "b", Keep: true, Confidence: 0.61, Reason: "maybe"},
	}
	got := applyKeepThreshold(items, parsed, 0.70)
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if !got[0].Keep || got[0].ThresholdRejected {
		t.Fatalf("high confidence should keep: %+v", got[0])
	}
	if got[1].Keep || !got[1].ThresholdRejected || got[1].Reason != "maybe" {
		t.Fatalf("low confidence should flip: %+v", got[1])
	}
	if got[2].Keep || got[2].ThresholdRejected || got[2].ArticleID != "c" {
		t.Fatalf("missing verdict should be skip/unsure: %+v", got[2])
	}
}
