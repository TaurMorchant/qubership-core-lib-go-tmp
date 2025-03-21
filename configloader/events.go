package configloader

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	InitedEventT EventT = iota + 1
	RefreshedEventT
)

type EventT int

type Event struct {
	Type EventT
	Data interface{}
}

type SubscriptionId struct {
	name string
}

type EventHandler func(e Event) error

var (
	ErrCannotFindSubscriber = errors.New("cannot find subscriber with given id")

	subscribers = &subscribersRegistry{
		registry: make(map[SubscriptionId]EventHandler, 10),
		notifyCh: make(chan Event, 10),
	}
)

func init() {
	subscribers.spawnListener()
}

func Subscribe(h EventHandler) (SubscriptionId, error) {
	subscribers.Lock()
	defer subscribers.Unlock()
	id := subscribers.generateId()
	subscribers.registry[id] = h
	return id, nil
}

func Unsubscribe(id SubscriptionId) error {
	subscribers.Lock()
	defer subscribers.Unlock()
	if _, ok := subscribers.registry[id]; ok {
		delete(subscribers.registry, id)
		return nil
	}
	return ErrCannotFindSubscriber
}

type subscribersRegistry struct {
	registry map[SubscriptionId]EventHandler
	notifyCh chan Event
	sync.RWMutex
	eventsCounter int32
}

func (s *subscribersRegistry) generateId() SubscriptionId {
	return SubscriptionId{name: fmt.Sprintf("sub-%d-%v", len(s.registry), time.Now().Format(time.RFC3339))}
}

func (s *subscribersRegistry) regCopy() map[SubscriptionId]EventHandler {
	s.RLock()
	defer s.RUnlock()
	regCopy := make(map[SubscriptionId]EventHandler, len(s.registry))
	for k := range s.registry {
		regCopy[k] = s.registry[k]
	}
	return regCopy
}

func (s *subscribersRegistry) notify(e Event) {
	s.notifyCh <- e
	atomic.AddInt32(&s.eventsCounter, 1)
}

func (s *subscribersRegistry) spawnListener() {
	go func() {
		for event := range s.notifyCh {
			log.Println("get event ", event)
			atomic.AddInt32(&s.eventsCounter, -1)
			for _, handlerF := range s.regCopy() {
				_ = handlerF(event)
			}
			log.Println("event handled ", event)
		}
	}()
}
