package medha

import (
	"fmt"
	"sync"
	"time"
)

func (e *Engine) notify(tenant string) {
	for ch := range e.subscribers[tenant] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
func (e *Engine) Notify(tenant string) { e.mu.Lock(); defer e.mu.Unlock(); e.notify(tenant) }
func (e *Engine) Subscribe(tenant string) (<-chan struct{}, func()) {
	e.mu.Lock()
	if e.subscribers == nil {
		e.subscribers = map[string]map[chan struct{}]struct{}{}
	}
	if e.subscribers[tenant] == nil {
		e.subscribers[tenant] = map[chan struct{}]struct{}{}
	}
	ch := make(chan struct{}, 1)
	e.subscribers[tenant][ch] = struct{}{}
	e.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			e.mu.Lock()
			defer e.mu.Unlock()
			delete(e.subscribers[tenant], ch)
			if len(e.subscribers[tenant]) == 0 {
				delete(e.subscribers, tenant)
			}
		})
	}
}
func (e *Engine) RestoreLastSeen(tenant, collector string, at time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n := e.nodes[tenant+":"+collector]; n != nil {
		n.seen = at
		n.LastSeen = stamp(at)
	}
}
func (e *Engine) RestoreIncidents(items []Explanation) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.incidents = map[string][]Explanation{}
	e.events = map[string][]Event{}
	for _, n := range e.nodes {
		n.incident = ""
	}
	for _, v := range items {
		e.incidents[v.Tenant] = append(e.incidents[v.Tenant], v)
		var id uint64
		fmt.Sscanf(v.ID, "incident-%d", &id)
		if id > e.next {
			e.next = id
		}
		if n := e.nodes[v.Tenant+":"+v.Collector]; n != nil && v.Status == "ACTIVE" {
			n.incident = v.ID
		}
	}
}
