package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const gracefulShutdownTimeout = 10 * time.Second

func (app *application) serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	serverError := make(chan error, 1)
	var aiWorkerDone <-chan struct{}

	if app.aiWorker != nil {
		workerDone := make(chan struct{})
		aiWorkerDone = workerDone

		go func() {
			defer close(workerDone)
			app.aiWorker.Run(ctx)
		}()
	}

	go func() {
		serverError <- server.Serve(listener)
	}()

	select {
	case err := <-serverError:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		app.infoLog.Print("shutting down server")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	if err := <-serverError; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	if app.notifications != nil {
		app.infoLog.Print("draining notification queue")

		if err := app.notifications.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf(
				"shutdown notification dispatcher: %w",
				err,
			)
		}
	}

	if aiWorkerDone != nil {
		select {
		case <-aiWorkerDone:
		case <-shutdownCtx.Done():
			return errors.New("shutdown AI worker: timed out")
		}
	}

	app.infoLog.Print("server stopped")

	return nil
}
