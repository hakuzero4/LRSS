package appsvc

import (
	"testing"
	"time"
)

func TestSubscribeLLMStream(t *testing.T) {
	got := make(chan LLMStreamEvent, 1)
	unsub := SubscribeLLMStream(func(ev LLMStreamEvent) {
		got <- ev
	})
	defer unsub()

	emitLLMStream(LLMStreamEvent{Feature: "chat", Text: "hi", Done: true})
	select {
	case ev := <-got:
		if ev.Feature != "chat" || ev.Text != "hi" || !ev.Done {
			t.Fatalf("got %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("listener did not receive stream event")
	}

	unsub()
	emitLLMStream(LLMStreamEvent{Feature: "chat", Text: "after"})
	select {
	case ev := <-got:
		t.Fatalf("unsubscribed listener still received %+v", ev)
	case <-time.After(40 * time.Millisecond):
	}
}
