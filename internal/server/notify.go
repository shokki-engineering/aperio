// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// notify.go tells departments and guests about service requests through the
// notifier [notify] names (internal/notify): the department when a request is
// raised, the guest when it is done. Deliveries run in the background and
// never fail a request; Stop waits for those already started.

import (
	"github.com/shokki-engineering/aperio/internal/config"
	"github.com/shokki-engineering/aperio/internal/notify"
	"github.com/shokki-engineering/aperio/internal/telemetry"
)

// useNotify has the operations service tell people through cfg's notifier.
func (s *Services) useNotify(cfg config.NotifyConfig) error {
	notifier, err := notify.New(cfg, telemetry.Info)
	if err != nil {
		return err
	}
	s.sender = notify.NewSender(notifier, cfg.Timeout(), telemetry.Meter(), telemetry.Warn)
	return s.Operations.NotifyThrough(s.sender, cfg.Departments)
}

// waitNotify waits for the deliveries already started.
func (s *Services) waitNotify() {
	if s.sender != nil {
		s.sender.Wait()
	}
}
