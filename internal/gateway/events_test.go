package gateway

import (
	"testing"

	"github.com/pmarkun/teendns/internal/policy"
)

func TestEventBufferKeepsOnlyAggregates(t *testing.T) {
	buffer := NewEventBuffer()
	buffer.Write(Event{ProfileID: "home", Query: "private.example", Action: policy.ActionBlock, Category: "gambling"})
	buffer.Write(Event{ProfileID: "home", Query: "another.example", Action: policy.ActionObserve, Category: "tracking"})

	summary := buffer.Summary("home")
	if summary.Total != 2 || summary.Blocked != 1 || summary.Observed != 1 || len(summary.Categories) != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
