package lemon

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestShutdownWithSignal(t *testing.T) {

	kill := 200 * time.Millisecond
	e, err := New()
	if err != nil {
		t.Fatalf("An error wasn't expected: %s", err)
	}

	h1 := &testHook{}
	h1.kill = make(chan struct{}, 1)

	h2 := &testHook{}
	h2.kill = make(chan struct{}, 1)

	h3 := &testHook{}
	h3.kill = make(chan struct{}, 1)

	e.interrupt = make(chan os.Signal, 1)

	e.Register(h1)
	e.Register(h2)
	e.Register(h3)

	c := make(chan struct{}, 1)

	go func() {
		time.Sleep(kill)
		e.interrupt <- syscall.SIGINT
	}()

	go func() {

		t0 := time.Now()
		if err = e.Start(); err != nil {
			t.Errorf("An error wasn't expected: %s", err)
		}

		delta := time.Since(t0)
		latency := (delta - kill)

		defer func() {
			c <- struct{}{}
		}()

		inDelta(t, latency, (10 * time.Millisecond), "Latency between signal and stop is too great")
		inEpsilon(t, delta, kill, (20 * time.Millisecond), "Engine shouldn't stopped in this interval")

		hasACompleteLifecycle(t, h1, "h1")
		hasACompleteLifecycle(t, h2, "h2")
		hasACompleteLifecycle(t, h3, "h3")

		t.Logf("Latency: %s", latency)

	}()

	select {
	case <-c:
		t.Log("Engine has stopped.")
	case <-time.After(600 * time.Millisecond):
		t.Fatal("Engine should have stopped.")
	}

}

func TestShutdownWithCancelContext(t *testing.T) {

	kill := 500 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), kill)

	e, err := NewWithContext(ctx)
	if err != nil {
		cancel()
		t.Fatalf("An error wasn't expected: %s", err)
	}

	h1 := &testHook{}
	h1.kill = make(chan struct{}, 1)

	h2 := &testHook{}
	h2.kill = make(chan struct{}, 1)

	h3 := &testHook{}
	h3.kill = make(chan struct{}, 1)

	c := make(chan struct{}, 1)

	e.Register(h1)
	e.Register(h2)
	e.Register(h3)

	go func() {

		t0 := time.Now()
		if err = e.Start(); err != nil {
			t.Errorf("An error wasn't expected: %s", err)
		}

		delta := time.Since(t0)
		latency := (delta - kill)

		defer func() {
			c <- struct{}{}
		}()

		inDelta(t, latency, (10 * time.Millisecond), "Latency between signal and stop is too great")
		inEpsilon(t, delta, kill, (20 * time.Millisecond), "Engine shouldn't stopped in this interval")

		hasACompleteLifecycle(t, h1, "h1")
		hasACompleteLifecycle(t, h2, "h2")
		hasACompleteLifecycle(t, h3, "h3")

		t.Logf("Latency: %s", latency)

	}()

	select {
	case <-c:
		t.Log("Engine has stopped.")
	case <-time.After(600 * time.Millisecond):
		t.Fatal("Engine should have stopped.")
	}

	cancel()

}

func TestShutdownWithHookError(t *testing.T) {

	e, err := New()
	if err != nil {
		t.Fatalf("An error wasn't expected: %s", err)
	}

	h1 := &testHook{}
	h1.kill = make(chan struct{}, 1)

	h2 := &testHook{}
	h2.startError = errors.New("An error has occurred: foobar")

	h3 := &testHook{}
	h3.kill = make(chan struct{}, 1)

	e.Register(h1)
	e.Register(h2)
	e.Register(h3)

	c := make(chan struct{}, 1)

	go func() {

		t0 := time.Now()
		if err = e.Start(); err == nil {
			t.Error("An error was expected")
		}

		delta := time.Since(t0)

		defer func() {
			c <- struct{}{}
		}()

		inDelta(t, delta, (20 * time.Millisecond), "Engine took way too long to shutdown")

		hasACompleteLifecycle(t, h1, "h1")
		hasStarted(t, h2, "h2")
		hasACompleteLifecycle(t, h3, "h3")

		t.Logf("Shutdown was successful.")

	}()

	select {
	case <-c:
		t.Log("Engine has stopped.")
	case <-time.After(600 * time.Millisecond):
		t.Fatal("Engine should have stopped.")
	}

}

func TestShutdownWithHookPanicOnStart(t *testing.T) {

	e, err := New()
	if err != nil {
		t.Fatalf("An error wasn't expected: %s", err)
	}

	h1 := &testHook{}
	h1.kill = make(chan struct{}, 1)

	h2 := &panicHook{}
	h2.panicOnStart = true
	h2.kill = make(chan struct{}, 1)

	h3 := &testHook{}
	h3.kill = make(chan struct{}, 1)

	e.Register(h1)
	e.Register(h2)
	e.Register(h3)

	c := make(chan struct{}, 1)

	go func() {

		t0 := time.Now()
		err = e.Start()

		if err == nil {
			t.Error("An error was expected")
		}

		if err.Error() != "lemon startup failed: Hook has a crashed: 0xDEADC0DE" {
			t.Errorf("Unexpected error: %s", err)
		}

		delta := time.Since(t0)

		defer func() {
			c <- struct{}{}
		}()

		inDelta(t, delta, (20 * time.Millisecond), "Engine took way too long to shutdown")

		hasACompleteLifecycle(t, h1, "h1")
		hasACompleteLifecycle(t, h3, "h3")

		if !h2.startCalled {
			t.Fatalf("Hook h2 should have been started.")
		}

		if h2.stopCalled {
			t.Fatalf("Hook h2 shouldn't have a shutdown request.")
		}

		t.Logf("Shutdown was successful.")

	}()

	select {
	case <-c:
		t.Log("Engine has stopped.")
	case <-time.After(600 * time.Millisecond):
		t.Fatal("Engine should have stopped.")
	}

}

func TestShutdownWithHookPanicOnStop(t *testing.T) {

	kill := 200 * time.Millisecond
	e, err := New(Timeout(kill))
	if err != nil {
		t.Fatalf("An error wasn't expected: %s", err)
	}

	h1 := &testHook{}
	h1.kill = make(chan struct{}, 1)

	h2 := &panicHook{}
	h2.panicOnStop = true
	h2.kill = make(chan struct{}, 1)

	h3 := &testHook{}
	h3.kill = make(chan struct{}, 1)

	e.Register(h1)
	e.Register(h2)
	e.Register(h3)

	c := make(chan struct{}, 1)

	go func() {
		time.Sleep(kill)
		e.interrupt <- syscall.SIGINT
	}()

	go func() {

		t0 := time.Now()
		if err = e.Start(); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}

		delta := time.Since(t0)
		end := kill * 2

		defer func() {
			c <- struct{}{}
		}()

		inEpsilon(t, delta, end, (20 * time.Millisecond), "Engine took way too long to shutdown")

		hasACompleteLifecycle(t, h1, "h1")
		hasACompleteLifecycle(t, h3, "h3")

		if !h2.stopCalled {
			t.Fatal("Hook h2 should have try to shutdown.")
		}

		t.Logf("Shutdown was successful.")

	}()

	select {
	case <-c:
		t.Log("Engine has stopped.")
	case <-time.After(600 * time.Millisecond):
		t.Fatal("Engine should have stopped.")
	}

}

func TestShutdownWithoutNew(t *testing.T) {

	kill := 200 * time.Millisecond
	e := &Engine{}

	h1 := &testHook{}
	h1.kill = make(chan struct{}, 1)

	h2 := &testHook{}
	h2.kill = make(chan struct{}, 1)

	h3 := &testHook{}
	h3.kill = make(chan struct{}, 1)

	e.interrupt = make(chan os.Signal, 1)

	e.Register(h1)
	e.Register(h2)
	e.Register(h3)

	c := make(chan struct{}, 1)

	go func() {
		time.Sleep(kill)
		e.interrupt <- syscall.SIGINT
	}()

	go func() {

		t0 := time.Now()
		if err := e.Start(); err != nil {
			t.Fatalf("An error wasn't expected: %s", err)
		}

		delta := time.Since(t0)
		latency := (delta - kill)

		defer func() {
			c <- struct{}{}
		}()

		inDelta(t, latency, (10 * time.Millisecond), "Latency between signal and stop is too great")
		inEpsilon(t, delta, kill, (20 * time.Millisecond), "Engine shouldn't stopped in this interval")

		hasACompleteLifecycle(t, h1, "h1")
		hasACompleteLifecycle(t, h2, "h2")
		hasACompleteLifecycle(t, h3, "h3")

		t.Logf("Latency: %s", latency)

	}()

	select {
	case <-c:
		t.Log("Engine has stopped.")
	case <-time.After(600 * time.Millisecond):
		t.Fatal("Engine should have stopped.")
	}

}
