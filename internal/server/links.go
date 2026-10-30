// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// links.go gives guests what they open without signing in: the links
// ShareQuote, ShareReservation and ShareInvoice sign (auth.Links), to pages
// of the guest site [auth] link_base_url names, and the advance a confirmed
// stay asks for ([concierge] advance_percent), billed through the finance
// service with its payment link.

import (
	"github.com/shokki-engineering/aperio/internal/auth"
	"github.com/shokki-engineering/aperio/internal/config"
)

// useLinks signs the share methods' links with links, to pages under
// cfg.LinkBaseURL, and has the concierge bill a confirmed stay's advance.
func (s *Services) useLinks(links *auth.Links, cfg config.AuthConfig, concierge config.ConciergeConfig) {
	s.Sales.ShareLinksWith(links, cfg.LinkBaseURL)
	s.Stays.ShareLinksWith(links, cfg.LinkBaseURL)
	s.Finance.ShareLinksWith(links, cfg.LinkBaseURL)
	s.Concierge.BillWith(s.Finance, concierge.Advance())
}
