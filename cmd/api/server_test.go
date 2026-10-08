package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeGracefullyShutsDown(t *testing.T) {
	t.Parallel()

	requestStarted := make(chan struct{})
	finishRequest := make(chan struct{})

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(requestStarted)
			<-finishRequest
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	app := &application{infoLog: log.New(io.Discard, "", 0)}
	ctx, cancel := context.WithCancel(context.Background())
	serveError := make(chan error, 1)

	go func() {
		serveError <- app.serve(ctx, server, listener)
	}()

	responseError := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			responseError <- err
			return
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusNoContent {
			responseError <- errors.New("unexpected response status")
			return
		}

		responseError <- nil
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not reach the handler")
	}

	cancel()
	close(finishRequest)

	select {
	case err := <-responseError:
		if err != nil {
			t.Fatalf("in-flight request failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("in-flight request did not finish")
	}

	select {
	case err := <-serveError:
		if err != nil {
			t.Fatalf("serve returned an error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestServeReturnsListenerErrors(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	app := &application{infoLog: log.New(io.Discard, "", 0)}
	server := &http.Server{Handler: http.NewServeMux()}

	err = app.serve(context.Background(), server, listener)
	if err == nil {
		t.Fatal("serve returned nil; want a listener error")
	}
}
