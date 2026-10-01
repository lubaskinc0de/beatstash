// Package stall cuts off downloads whose source went silent.
package stall

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

var errStalled = errors.New("download stalled")

// Watch cuts off the bodies that went silent for Timeout.
type Watch struct {
	Timeout time.Duration
	// AfterFunc is time.AfterFunc unless the time is the caller's.
	AfterFunc func(d time.Duration, f func()) Timer
}

// Timer is what Watch needs of a *time.Timer.
type Timer interface {
	Reset(d time.Duration) bool
	Stop() bool
}

func RealTime(d time.Duration, f func()) Timer {
	return time.AfterFunc(d, f)
}

// Do sends the request; its response body ends the request once no byte
// comes for the Timeout. Only the body is watched: whatever waits before the
// request is none of its business.
func (w Watch) Do(client *http.Client, req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	resp, err := client.Do(req.WithContext(ctx)) //nolint:gosec // G704: callers build the URL from their Provider
	if err != nil {
		cancel()
		return nil, err
	}
	body := &body{ReadCloser: resp.Body, cancel: cancel, timeout: w.Timeout}
	body.timer = w.AfterFunc(w.Timeout, body.cutOff)
	resp.Body = body
	return resp, nil
}

type body struct {
	io.ReadCloser
	cancel  context.CancelFunc
	timeout time.Duration
	timer   Timer
	stalled atomic.Bool
}

func (b *body) cutOff() {
	b.stalled.Store(true)
	b.cancel()
}

func (b *body) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.timer.Reset(b.timeout)
	}
	if err != nil && b.stalled.Load() {
		err = fmt.Errorf("%w: nothing for %s", errStalled, b.timeout)
	}
	return n, err
}

func (b *body) Close() error {
	b.timer.Stop()
	defer b.cancel()
	return b.ReadCloser.Close()
}
