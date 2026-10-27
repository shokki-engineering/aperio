// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

// sender.go is the Sender: what a service hands its messages to. It delivers
// each one in the background, so a slow channel never holds up the request
// that caused it, and a failure is the Sender's to log and count -- never the
// caller's error: a pot of coffee is not refused because WhatsApp is down.

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/the-protobuf-project/runtime-go/telemetry"
)

// Sender delivers messages through a Notifier without failing its callers.
type Sender struct {
	n        Notifier
	timeout  time.Duration
	warn     func(msg string, kv ...any)
	failures telemetry.Counter
	failed   atomic.Int64
	wg       sync.WaitGroup
}

// NewSender delivers through n, each delivery bounded by timeout (10s when
// 0); warn logs a failure and m counts them (nil logs or counts nothing).
func NewSender(n Notifier, timeout time.Duration, m telemetry.Meter, warn func(msg string, kv ...any)) *Sender {
	if m == nil {
		m = telemetry.NoopMeter
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Sender{n: n, timeout: timeout, warn: warn,
		failures: m.Counter("aperio.notify.failures", telemetry.WithUnit("1"),
			telemetry.WithDescription("Notifications that could not be delivered, by event."))}
}

// Send delivers m in the background, detached from ctx's cancellation so
// the caller returning does not call it off. A Sender without a Notifier, or
// a message without a recipient, sends nothing.
func (s *Sender) Send(ctx context.Context, m Message) {
	if s == nil || s.n == nil || m.To == "" {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
		defer cancel()
		if err := s.n.Notify(ctx, m); err != nil {
			s.failed.Add(1)
			s.failures.Add(ctx, 1, telemetry.Labels{"event": m.Event})
			if s.warn != nil {
				s.warn("notification not delivered", "event", m.Event, "subject", m.Subject, "err", err)
			}
		}
	}()
}

// Wait waits for the deliveries under way: at shutdown, and in tests.
func (s *Sender) Wait() {
	if s != nil {
		s.wg.Wait()
	}
}

// Failed is how many deliveries have failed since the Sender was made.
func (s *Sender) Failed() int64 {
	if s == nil {
		return 0
	}
	return s.failed.Load()
}
