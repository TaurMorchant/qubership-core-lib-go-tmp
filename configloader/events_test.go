package configloader

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"sync"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func cleanupSubscribersRegistry(t *testing.T) {
	t.Logf("cleanupSubscribersRegistry2 start")
	subscribers.Lock()
	for k := range subscribers.registry {
		delete(subscribers.registry, k)
	}
	subscribers.Unlock()
	t.Logf("cleanupSubscribersRegistry2 finish")
}

func TestSubscribe_OnInitEvent(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	var gotEvent1, gotEvent2 Event
	over1, over2 := make(chan struct{}), make(chan struct{})
	id1, err := Subscribe(func(e Event) error {
		gotEvent1 = e
		close(over1)
		return nil
	})
	assert.Nil(t, err)
	id2, err := Subscribe(func(e Event) error {
		gotEvent2 = e
		close(over2)
		return nil
	})
	assert.Nil(t, err)
	assert.NotEqual(t, id1, id2)

	Init()
	<-over1
	<-over2
	assert.Equal(t, InitedEventT, gotEvent1.Type)
	assert.Equal(t, InitedEventT, gotEvent2.Type)
}

func TestUnsubscribeOnExistentHandler(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	id, err := Subscribe(func(Event) error { return nil })
	assert.Nil(t, err)
	assert.NotEmpty(t, id.name)

	err = Unsubscribe(id)
	assert.Nil(t, err)
}

func TestUnsubscribeOnNonExistentHandler(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	err := Unsubscribe(SubscriptionId{name: "non-existent"})
	assert.NotNil(t, err)
	assert.Equal(t, ErrCannotFindSubscriber, err)
}

func TestNotifyWhenNoSubscribers(t *testing.T) {
	t.Logf("TestNotifyWhenNoSubscribers start")
	defer cleanupSubscribersRegistry(t)
	assert.Empty(t, subscribers.registry)
	subscribers.notify(Event{Type: InitedEventT, Data: "TestNotifyWhenNoSubscribers"})
	t.Logf("TestNotifyWhenNoSubscribers finish")
}

func TestNotifyNotConflictsWithUnSubscribe(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	// concurrent read test

	assert.NotPanics(t, func() {
		testOver := make(chan struct{})
		notifyOver := make(chan struct{})
		go func() {
			for {
				select {
				case <-testOver:
					close(notifyOver)
					return
				default:
					subscribers.notify(Event{Type: RefreshedEventT})
				}
			}
		}()

		for i := 0; i < 100; i++ {
			id, err := Subscribe(func(e Event) error { return nil })
			assert.Nil(t, err)
			err = Unsubscribe(id)
			assert.Nil(t, err)
		}
		close(testOver)
		<-notifyOver
	})
}

func TestDataAtEventParamIsPossible(t *testing.T) {
	defer cleanupSubscribersRegistry(t)

	var notImplementedEventT EventT = -1
	type dataCorrespondsToNotImplementedEventT struct {
		val string
	}
	over := make(chan struct{})

	handlerF := func(e Event) error {
		var gotData dataCorrespondsToNotImplementedEventT
		t.Logf("Got event %v in handler\n", e)
		if e.Type == notImplementedEventT {
			if e.Data != nil {
				gotData = e.Data.(dataCorrespondsToNotImplementedEventT)
			}
		}
		assert.Equal(t, "param-val", gotData.val)
		close(over)
		return nil
	}

	_, err := Subscribe(handlerF)
	assert.Nil(t, err)
	subscribers.notify(Event{Type: notImplementedEventT, Data: dataCorrespondsToNotImplementedEventT{val: "param-val"}})
	<-over
}

func TestSubscribeRace(t *testing.T) {
	defer cleanupSubscribersRegistry(t)
	handler := func(e Event) error {
		return nil
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		id, _ := Subscribe(handler)
		Unsubscribe(id)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		id, _ := Subscribe(handler)
		Unsubscribe(id)
		wg.Done()
	}()
	wg.Wait()
}

func ExampleSubscribe() {
	handlerF := func(e Event) error {
		if e.Type == InitedEventT {
			fmt.Println("Config inited")
		} else if e.Type == RefreshedEventT {
			fmt.Println("Config refreshed")
		} else {
			fmt.Println("Unknown event sent")
		}
		return nil
	}
	id, err := Subscribe(handlerF)
	if err != nil {
		// error handling
	}

	_ = Refresh()

	if err := Unsubscribe(id); err != nil {
		// error handling
	}
	//
}
