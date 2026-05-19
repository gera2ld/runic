package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Broker struct {
	mu      sync.RWMutex
	subs    map[chan []byte]bool
}

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[chan []byte]bool),
	}
}

func (b *Broker) Subscribe() chan []byte {
	ch := make(chan []byte, 64)
	b.mu.Lock()
	b.subs[ch] = true
	b.mu.Unlock()
	return ch
}

func (b *Broker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.subs, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *Broker) Broadcast(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		fmt.Printf("[sse] marshal error: %v\n", err)
		return
	}
	b.mu.RLock()
	for ch := range b.subs {
		select {
		case ch <- data:
		default:
		}
	}
	b.mu.RUnlock()
}

func (b *Broker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "\n")
		flusher.Flush()

		ch := b.Subscribe()
		defer b.Unsubscribe(ch)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}
