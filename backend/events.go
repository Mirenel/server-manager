package main

import (
	"sync"
	"time"
)

const (
	EventStarted = "started"
	EventStopped = "stopped"
	EventCrashed = "crashed"
)

type Event struct {
	TimestampMS int64  `json:"timestamp_ms"`
	ProcessID   string `json:"process_id"`
	ProcessName string `json:"process_name"`
	Type        string `json:"type"`
}

type EventStore struct {
	events [500]Event
	head   int
	count  int
	mu     sync.Mutex
}

// Record adds a new event to the ring buffer
func (es *EventStore) Record(id, name, eventType string) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.count < len(es.events) {
		es.count++
	} else {
		es.head = (es.head + 1) % len(es.events)
	}

	idx := (es.head + es.count - 1) % len(es.events)
	es.events[idx] = Event{
		TimestampMS: time.Now().UnixMilli(),
		ProcessID:   id,
		ProcessName: name,
		Type:        eventType,
	}
}

// All returns all events in chronological order
func (es *EventStore) All() []Event {
	es.mu.Lock()
	defer es.mu.Unlock()

	result := make([]Event, es.count)
	for i := 0; i < es.count; i++ {
		idx := (es.head + i) % len(es.events)
		result[i] = es.events[idx]
	}
	return result
}
