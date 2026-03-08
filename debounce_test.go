package main

import (
	"testing"
	"time"
)

func TestDebouncer_SingleSignal(t *testing.T) {
	d := newDebouncer(100 * time.Millisecond)
	go d.run()

	// Drain the initial timer trigger (10ms startup)
	<-d.output

	d.signal()

	select {
	case <-d.output:
		// success — got trigger after delay
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected trigger, got timeout")
	}
}

func TestDebouncer_BatchesRapidSignals(t *testing.T) {
	d := newDebouncer(200 * time.Millisecond)
	go d.run()

	// Drain the initial timer trigger
	<-d.output

	// Fire 5 rapid signals over 100ms
	for i := 0; i < 5; i++ {
		d.signal()
		time.Sleep(20 * time.Millisecond)
	}

	// Should get exactly ONE trigger after the delay
	select {
	case <-d.output:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected trigger after batching, got timeout")
	}

	// Should NOT get a second trigger
	select {
	case <-d.output:
		t.Fatal("got unexpected second trigger — debouncer didn't batch properly")
	case <-time.After(300 * time.Millisecond):
		// success — no extra triggers
	}
}

func TestDebouncer_ResetsOnNewSignal(t *testing.T) {
	d := newDebouncer(150 * time.Millisecond)
	go d.run()

	// Drain the initial timer trigger
	<-d.output

	d.signal()
	time.Sleep(100 * time.Millisecond) // wait 100ms (timer at 150ms)
	d.signal()                          // reset — timer starts over

	// Should NOT fire after just 50ms more (would have been 150ms from first signal)
	select {
	case <-d.output:
		t.Fatal("triggered too early — timer was not reset")
	case <-time.After(100 * time.Millisecond):
		// good, hasn't fired yet
	}

	// Should fire within the next 100ms (150ms from second signal)
	select {
	case <-d.output:
		// success — fired after the reset delay
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected trigger after reset delay")
	}
}

func TestDebouncer_InitialTrigger(t *testing.T) {
	d := newDebouncer(100 * time.Millisecond)
	go d.run()

	// Should get an initial trigger almost immediately (10ms timer)
	select {
	case <-d.output:
		// success — initial build trigger fired
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected initial trigger on startup")
	}
}
