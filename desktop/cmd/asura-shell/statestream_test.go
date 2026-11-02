package main

import (
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestEventTopicPreservesBurstOrder(t *testing.T) {
	topic := newEventTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)

	published := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			topic.publish([]byte(strconv.Itoa(i)))
		}
		close(published)
	}()

	for want := 0; want < 100; want++ {
		select {
		case frame := <-sub.frames:
			if got := string(frame); got != strconv.Itoa(want) {
				t.Fatalf("event %d = %q", want, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for event %d", want)
		}
	}
	<-published
}

func TestEventTopicRetainsIdenticalEvents(t *testing.T) {
	topic := newEventTopic()
	sub := topic.subscribe()
	defer topic.unsubscribe(sub)

	topic.publish([]byte("same"))
	topic.publish([]byte("same"))
	for i := 0; i < 2; i++ {
		select {
		case frame := <-sub.frames:
			if string(frame) != "same" {
				t.Fatalf("event %d = %q", i, frame)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for duplicate event %d", i)
		}
	}
}

func TestEventTopicDoesNotReplayBeforeSubscription(t *testing.T) {
	topic := newEventTopic()
	topic.publish([]byte("old"))

	sub := topic.subscribe()
	defer topic.unsubscribe(sub)

	topic.publish([]byte("new"))
	select {
	case frame := <-sub.frames:
		if string(frame) != "new" {
			t.Fatalf("event = %q, want new", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for new event")
	}
}

// The key overlay's evdev reader must not outlive the shell that asked for it:
// when the last keypress subscriber goes, the backend is switched off.
func TestKeypressBackendStopsWhenLastSubscriberLeaves(t *testing.T) {
	dir := t.TempDir()
	d := &daemon{quit: make(chan struct{})}
	d.keypress = newKeypressManager(d.registerEventTopic("keypress"), dir, dir, filepath.Join(dir, "settings.json"))
	d.keypress.configure(true, "all")
	defer d.keypress.configure(false, "all")

	server, client := net.Pipe()
	served := make(chan struct{})
	go func() { d.serveSubscription(server, "subscribe keypress"); close(served) }()
	_ = client.Close()
	select {
	case <-served:
	case <-time.After(3 * time.Second):
		t.Fatal("the subscription did not end when its client left")
	}

	d.keypress.mu.Lock()
	enabled := d.keypress.enabled
	d.keypress.mu.Unlock()
	if enabled {
		t.Fatal("the key reader kept running with nobody subscribed")
	}
}
