package notifications

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/SaadShakeel1/TaskFlow/internal/mailer"
)

var (
	ErrQueueFull        = errors.New("notification queue is full")
	ErrDispatcherClosed = errors.New("notification dispatcher is closed")
)

const (
	defaultQueueCapacity = 20
	defaultWorkerCount   = 1
	defaultMaxAttempts   = 3
	defaultRetryDelay    = 500 * time.Millisecond
)

type Dispatcher struct {
	sender      mailer.Sender
	errorLog    *log.Logger
	jobs        chan mailer.Email
	maxAttempts int
	retryDelay  time.Duration

	mu           sync.Mutex
	closed       bool
	workers      sync.WaitGroup
	shutdownOnce sync.Once
	stopped      chan struct{}
}

func New(sender mailer.Sender, errorLog *log.Logger) *Dispatcher {
	return newDispatcher(
		sender,
		errorLog,
		defaultQueueCapacity,
		defaultWorkerCount,
		defaultMaxAttempts,
		defaultRetryDelay,
	)
}

func newDispatcher(sender mailer.Sender, errorLog *log.Logger, queueCapacity int, workerCount int, maxAttempts int, retryDelay time.Duration) *Dispatcher {

	dispatcher := &Dispatcher{
		sender:      sender,
		errorLog:    errorLog,
		jobs:        make(chan mailer.Email, queueCapacity),
		maxAttempts: maxAttempts,
		retryDelay:  retryDelay,
		stopped:     make(chan struct{}),
	}

	dispatcher.workers.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go dispatcher.run()
	}

	return dispatcher
}

func (d *Dispatcher) run() {
	defer d.workers.Done()

	for email := range d.jobs {
		d.deliver(email)
	}
}

func (d *Dispatcher) Enqueue(email mailer.Email) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return ErrDispatcherClosed
	}

	select {
	case d.jobs <- email:
		return nil
	default:
		return ErrQueueFull
	}
}

func (d *Dispatcher) deliver(email mailer.Email) {
	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		err := d.sender.Send(email)

		if err == nil {
			return
		}

		if attempt == d.maxAttempts {
			d.errorLog.Printf(
				"send notification to %q failed after %d attempts: %v",
				email.To,
				attempt,
				err,
			)
			return
		}
		time.Sleep(d.retryDelay * time.Duration(attempt))
	}
}

func (d *Dispatcher) Shutdown(ctx context.Context) error {
	d.shutdownOnce.Do(func() {
		d.mu.Lock()
		d.closed = true
		close(d.jobs)
		d.mu.Unlock()

		go func() {
			d.workers.Wait()
			close(d.stopped)
		}()
	})
	select {
	case <-d.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

}
