package eventbus

import (
	"reflect"
	"sync"
)

type EventBus struct {
	subscribers map[reflect.Type][]chan interface{}
	mu          sync.Mutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[reflect.Type][]chan interface{}),
	}
}

// Subscribe 订阅某个事件类型
func (eb *EventBus) Subscribe(eventType reflect.Type, ch chan interface{}) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if _, exists := eb.subscribers[eventType]; !exists {
		eb.subscribers[eventType] = []chan interface{}{}
	}
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
}

// Publish 发布事件
func (eb *EventBus) Publish(event interface{}) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eventType := reflect.TypeOf(event)
	if subs, exists := eb.subscribers[eventType]; exists {
		for _, sub := range subs {
			sub <- event
		}
	}
}
