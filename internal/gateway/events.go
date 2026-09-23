package gateway

import (
	"sort"
	"sync"
)

type MultiEventSink []EventSink

func (s MultiEventSink) Write(event Event) {
	for _, sink := range s {
		sink.Write(event)
	}
}

type EventSummary struct {
	ProfileID  string                 `json:"profile_id"`
	Total      int                    `json:"total"`
	Blocked    int                    `json:"blocked"`
	Observed   int                    `json:"observed"`
	Unknown    int                    `json:"unknown"`
	Categories []CategoryEventSummary `json:"categories"`
}

type CategoryEventSummary struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

// EventBuffer stores only aggregate counters. It deliberately does not retain
// query names, so the administrative interface cannot become browsing history.
type EventBuffer struct {
	mu       sync.RWMutex
	profiles map[string]*EventSummary
}

func NewEventBuffer() *EventBuffer {
	return &EventBuffer{profiles: make(map[string]*EventSummary)}
}

func (b *EventBuffer) Write(event Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	summary := b.profiles[event.ProfileID]
	if summary == nil {
		summary = &EventSummary{ProfileID: event.ProfileID}
		b.profiles[event.ProfileID] = summary
	}
	summary.Total++
	switch event.Action {
	case "block":
		summary.Blocked++
	case "observe":
		summary.Observed++
	}
	category := event.Category
	if category == "" {
		category = "unknown"
		summary.Unknown++
	}
	for index := range summary.Categories {
		if summary.Categories[index].Category == category {
			summary.Categories[index].Count++
			return
		}
	}
	summary.Categories = append(summary.Categories, CategoryEventSummary{Category: category, Count: 1})
}

func (b *EventBuffer) Summary(profileID string) EventSummary {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stored := b.profiles[profileID]
	if stored == nil {
		return EventSummary{ProfileID: profileID, Categories: []CategoryEventSummary{}}
	}
	result := *stored
	result.Categories = append([]CategoryEventSummary(nil), stored.Categories...)
	sort.Slice(result.Categories, func(i, j int) bool {
		return result.Categories[i].Count > result.Categories[j].Count
	})
	return result
}
