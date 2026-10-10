package notifications

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
)

type senderFunc func(mailer.Email) error

func (f senderFunc) Send(email mailer.Email) error {
	return f(email)
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func shutdownWithin(t *testing.T, dispatcher *Dispatcher) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := dispatcher.Shutdown(ctx); err != nil {
		t.Errorf("shutdown dispatcher: %v", err)
	}
}

func TestDispatcherDeliversQueuedEmail(t *testing.T) {
	delivered := make(chan mailer.Email, 1)

	dispatcher := newDispatcher(
		senderFunc(func(email mailer.Email) error {
			delivered <- email
			return nil
		}),
		testLogger(),
		1,
		1,
		1,
		0,
	)
	t.Cleanup(func() {
		shutdownWithin(t, dispatcher)
	})

	want := mailer.Email{
		To:      "assignee@example.com",
		Subject: "Task assigned",
		Body:    "You were assigned a task.",
	}

	if err := dispatcher.Enqueue(want); err != nil {
		t.Fatalf("enqueue email: %v", err)
	}

	select {
	case got := <-delivered:
		if got != want {
			t.Errorf("delivered email = %#v, want %#v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("email was not delivered")
	}
}

func TestDispatcherRetriesBeforeSuccess(t *testing.T) {
	var attempts atomic.Int32
	delivered := make(chan struct{})

	dispatcher := newDispatcher(
		senderFunc(func(email mailer.Email) error {
			attempt := attempts.Add(1)

			if attempt < 3 {
				return errors.New("temporary SMTP failure")
			}

			close(delivered)
			return nil
		}),
		testLogger(),
		1,
		1,
		3,
		0,
	)
	t.Cleanup(func() {
		shutdownWithin(t, dispatcher)
	})

	if err := dispatcher.Enqueue(mailer.Email{
		To:      "assignee@example.com",
		Subject: "Task assigned",
		Body:    "You were assigned a task.",
	}); err != nil {
		t.Fatalf("enqueue email: %v", err)
	}

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("email was not delivered after retries")
	}

	if got := attempts.Load(); got != 3 {
		t.Errorf("send attempts = %d, want 3", got)
	}
}

func TestDispatcherRejectsEmailWhenQueueIsFull(t *testing.T) {
	firstSendStarted := make(chan struct{})
	releaseFirstSend := make(chan struct{})
	var releaseOnce sync.Once
	var attempts atomic.Int32

	release := func() {
		releaseOnce.Do(func() {
			close(releaseFirstSend)
		})
	}

	dispatcher := newDispatcher(
		senderFunc(func(email mailer.Email) error {
			if attempts.Add(1) == 1 {
				close(firstSendStarted)
				<-releaseFirstSend
			}

			return nil
		}),
		testLogger(),
		1,
		1,
		1,
		0,
	)
	t.Cleanup(func() {
		release()
		shutdownWithin(t, dispatcher)
	})

	if err := dispatcher.Enqueue(mailer.Email{To: "first@example.com"}); err != nil {
		t.Fatalf("enqueue first email: %v", err)
	}

	select {
	case <-firstSendStarted:
	case <-time.After(time.Second):
		t.Fatal("worker did not start the first email")
	}

	if err := dispatcher.Enqueue(mailer.Email{To: "second@example.com"}); err != nil {
		t.Fatalf("enqueue second email: %v", err)
	}

	err := dispatcher.Enqueue(mailer.Email{To: "third@example.com"})
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("enqueue third email error = %v, want ErrQueueFull", err)
	}

	release()
	shutdownWithin(t, dispatcher)

	if got := attempts.Load(); got != 2 {
		t.Errorf("send attempts = %d, want 2", got)
	}
}

func TestDispatcherShutdownDrainsQueuedEmails(t *testing.T) {
	var delivered atomic.Int32

	dispatcher := newDispatcher(
		senderFunc(func(email mailer.Email) error {
			delivered.Add(1)
			return nil
		}),
		testLogger(),
		3,
		1,
		1,
		0,
	)

	for i := 0; i < 3; i++ {
		if err := dispatcher.Enqueue(mailer.Email{To: "assignee@example.com"}); err != nil {
			t.Fatalf("enqueue email %d: %v", i, err)
		}
	}

	shutdownWithin(t, dispatcher)

	if got := delivered.Load(); got != 3 {
		t.Errorf("delivered emails = %d, want 3", got)
	}
}

func TestDispatcherRejectsEmailAfterShutdown(t *testing.T) {
	dispatcher := newDispatcher(
		senderFunc(func(email mailer.Email) error {
			return nil
		}),
		testLogger(),
		1,
		1,
		1,
		0,
	)

	shutdownWithin(t, dispatcher)

	err := dispatcher.Enqueue(mailer.Email{To: "assignee@example.com"})
	if !errors.Is(err, ErrDispatcherClosed) {
		t.Fatalf("enqueue after shutdown error = %v, want ErrDispatcherClosed", err)
	}
}

func TestDispatcherShutdownRespectsContextDeadline(t *testing.T) {
	sendStarted := make(chan struct{})
	releaseSend := make(chan struct{})
	var releaseOnce sync.Once

	release := func() {
		releaseOnce.Do(func() {
			close(releaseSend)
		})
	}

	dispatcher := newDispatcher(
		senderFunc(func(email mailer.Email) error {
			close(sendStarted)
			<-releaseSend
			return nil
		}),
		testLogger(),
		1,
		1,
		1,
		0,
	)
	t.Cleanup(func() {
		release()
		shutdownWithin(t, dispatcher)
	})

	if err := dispatcher.Enqueue(mailer.Email{To: "assignee@example.com"}); err != nil {
		t.Fatalf("enqueue email: %v", err)
	}

	select {
	case <-sendStarted:
	case <-time.After(time.Second):
		t.Fatal("worker did not start sending")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := dispatcher.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error = %v, want context.DeadlineExceeded", err)
	}

	release()
	shutdownWithin(t, dispatcher)
}

func TestDispatcherLogsPermanentDeliveryFailure(t *testing.T) {
	var attempts atomic.Int32
	var logs bytes.Buffer

	dispatcher := newDispatcher(
		senderFunc(func(email mailer.Email) error {
			attempts.Add(1)
			return errors.New("SMTP unavailable")
		}),
		log.New(&logs, "", 0),
		1,
		1,
		3,
		0,
	)

	if err := dispatcher.Enqueue(mailer.Email{To: "assignee@example.com"}); err != nil {
		t.Fatalf("enqueue email: %v", err)
	}

	shutdownWithin(t, dispatcher)

	if got := attempts.Load(); got != 3 {
		t.Errorf("send attempts = %d, want 3", got)
	}

	if !strings.Contains(logs.String(), "failed after 3 attempts") {
		t.Errorf("log output = %q, want permanent failure message", logs.String())
	}
}
