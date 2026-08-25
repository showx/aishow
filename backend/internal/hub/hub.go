package hub

import (
	"encoding/json"
	"sync"
	"time"
)

type Event struct {
	Type string      `json:"type"`
	At   time.Time   `json:"at"`
	Data interface{} `json:"data"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func New() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *Hub) Broadcast(eventType string, data interface{}) {
	payload, err := json.Marshal(Event{
		Type: eventType,
		At:   time.Now(),
		Data: data,
	})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- payload:
		default:
		}
	}
}
