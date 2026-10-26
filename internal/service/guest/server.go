// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package guest serves aperio.guest.v1 -- contacts -- on the generated AIP
// server, with the rule the generated layer cannot keep: a contact's
// phone_numbers, the E.164 numbers of its phones as a column of their own,
// which is how a returning guest is found by the number they call or type
// (the concierge's). SummarizeContact, a guest's history, is summary.go.
package guest

import (
	"context"
	"slices"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/guest/v1/guestpbv1svc"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/guest/v1/guestpbv1"
	"google.golang.org/protobuf/proto"
)

// Server serves aperio.guest.v1.GuestService: the generated standard methods
// with the phone_numbers hooks, and SummarizeContact.
type Server struct {
	*guestpbv1svc.GuestServiceServer
	history History
}

// New returns the GuestService over repos. SummarizeContact needs the other
// services' (ReadHistoryFrom).
func New(repos guestpbv1svc.GuestServiceRepositories, opts ...servicex.Option) *Server {
	return &Server{GuestServiceServer: guestpbv1svc.NewGuestServiceServer(repos, &hooks{repos: repos}, opts...)}
}

// ReadHistoryFrom reads a guest's history from h: the services that are
// built after this one, since they read its contacts.
func (s *Server) ReadHistoryFrom(h History) { s.history = h }

// hooks keep phone_numbers in step with phones.
type hooks struct {
	guestpbv1svc.NopGuestServiceHooks

	repos guestpbv1svc.GuestServiceRepositories
}

// BeforeCreateContact sets the new contact's phone_numbers; Create stores it.
func (h *hooks) BeforeCreateContact(_ context.Context, _ string, m *guestpbv1.Contact) error {
	m.PhoneNumbers = PhoneNumbers(m)
	return nil
}

// AfterUpdateContact sets phone_numbers again when the phones changed.
func (h *hooks) AfterUpdateContact(ctx context.Context, out *guestpbv1.Contact) error {
	numbers := PhoneNumbers(out)
	if slices.Equal(numbers, out.GetPhoneNumbers()) {
		return nil
	}
	out.PhoneNumbers = numbers
	updated, err := h.repos.Contacts.UpdateOutputOnly(ctx, out, []string{"phone_numbers"})
	if err != nil {
		return err
	}
	proto.Reset(out)
	proto.Merge(out, updated)
	return nil
}

// PhoneNumbers are c's phone numbers as they are searched: each number
// without spaces, dashes or brackets, in order, once.
func PhoneNumbers(c *guestpbv1.Contact) []string {
	var out []string
	for _, p := range c.GetPhones() {
		n := Normalize(p.GetNumber())
		if n != "" && !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

// Normalize is a phone number as a caller or a form gives it -- "+91 98765
// 43210", "(0824) 222-1234" -- with only its digits and a leading plus.
func Normalize(number string) string {
	var b strings.Builder
	for i, r := range strings.TrimSpace(number) {
		if (r >= '0' && r <= '9') || (r == '+' && i == 0) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
