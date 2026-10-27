// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations

// requests.go checks a request as it is made, so that completing it later
// cannot fail on what was wrong from the start: a charge the venue's
// invoices cannot carry, or a booking that is not a stay or an event of the
// venue.

import (
	"context"
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
)

// BeforeCreateServiceRequest refuses a booking of another venue or of no
// kind that is billed, and a charge that is negative or not in the venue's
// currency.
func (h *hooks) BeforeCreateServiceRequest(ctx context.Context, parent string, m *operationspbv1.ServiceRequest) error {
	if b := m.GetBooking(); b != "" && !strings.HasPrefix(billed(b), parent+"/") {
		return servicex.BadField("INVALID_BOOKING", "service_request.booking",
			"the booking is not a reservation or an event of "+parent, "Name the stay (venues/{venue}/reservations/{id}) or event it is for")
	}
	c := m.GetCharge()
	if c == nil {
		return nil
	}
	if c.GetUnits() < 0 || c.GetNanos() < 0 {
		return servicex.BadField("NEGATIVE_CHARGE", "service_request.charge", "the charge is negative", "Charge zero or more")
	}
	v, err := h.venues.Get(ctx, parent)
	if err != nil {
		return servicex.NotFoundAs(err, venueType, parent)
	}
	if cur := c.GetCurrencyCode(); cur != v.GetCurrencyCode() {
		return servicex.BadField("CURRENCY_MISMATCH", "service_request.charge.currency_code",
			"the charge is in "+cur+", not the venue's "+v.GetCurrencyCode(), "Charge in "+v.GetCurrencyCode())
	}
	return nil
}
