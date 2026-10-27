// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package razorpay

// links.go makes payment links (POST /payment_links). Our request id is the
// link's reference_id, which Razorpay keeps unique: a second create with it
// is refused, and the link made the first time is read back by it instead,
// so a retried RequestPayment gets the same link. Razorpay's own SMS and
// email are off -- aperio tells the guest itself -- and partial payment is
// off, so the link is paid in full or not at all.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/shokki-engineering/aperio/internal/service/finance"
	"google.golang.org/genproto/googleapis/type/money"
)

// noteKey marks what aperio made, in a link's or refund's notes: the
// webhook acts on those, and acknowledges the rest of the account's.
const noteKey = "aperio"

// linkBody is a payment link to create.
type linkBody struct {
	Amount         int64             `json:"amount"`
	Currency       string            `json:"currency"`
	AcceptPartial  bool              `json:"accept_partial"`
	Description    string            `json:"description,omitempty"`
	Customer       *customer         `json:"customer,omitempty"`
	Notify         notify            `json:"notify"`
	ReminderEnable bool              `json:"reminder_enable"`
	ExpireBy       int64             `json:"expire_by"`
	ReferenceID    string            `json:"reference_id"`
	Notes          map[string]string `json:"notes"`
}

type customer struct {
	Name    string `json:"name,omitempty"`
	Contact string `json:"contact,omitempty"`
	Email   string `json:"email,omitempty"`
}

type notify struct {
	SMS   bool `json:"sms"`
	Email bool `json:"email"`
}

// link is a payment link as Razorpay returns it.
type link struct {
	ID       string `json:"id"`
	ShortURL string `json:"short_url"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	ExpireBy int64  `json:"expire_by"`
	Payments []struct {
		PaymentID string `json:"payment_id"`
		Status    string `json:"status"`
	} `json:"payments"`
}

// Link makes the payment link r asks for, or returns the one already made
// for r.Reference.
func (c *Client) Link(ctx context.Context, r finance.LinkRequest) (finance.Link, error) {
	body := linkBody{
		Amount: paise(r.Amount), Currency: r.Amount.GetCurrencyCode(), Description: r.Description,
		ExpireBy: r.Expire.Unix(), ReferenceID: r.Reference, Notes: map[string]string{noteKey: r.Invoice},
	}
	if r.Name != "" || r.Phone != "" || r.Email != "" {
		body.Customer = &customer{Name: r.Name, Contact: r.Phone, Email: r.Email}
	}
	var l link
	err := c.do(ctx, http.MethodPost, "/payment_links", body, &l)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusBadRequest {
		var found struct {
			PaymentLinks []link `json:"payment_links"`
		}
		if c.do(ctx, http.MethodGet, "/payment_links?reference_id="+url.QueryEscape(r.Reference), nil, &found) == nil &&
			len(found.PaymentLinks) > 0 {
			l, err = found.PaymentLinks[0], nil
		}
	}
	if err != nil {
		return finance.Link{}, err
	}
	return finance.Link{ID: l.ID, URL: l.ShortURL, Amount: fromPaise(l.Amount, l.Currency), Expire: time.Unix(l.ExpireBy, 0)}, nil
}

// paise is m in its currency's minor unit, as Razorpay counts amounts.
func paise(m *money.Money) int64 {
	return m.GetUnits()*100 + int64(m.GetNanos())/10_000_000
}

// fromPaise is n minor units of currency.
func fromPaise(n int64, currency string) *money.Money {
	return &money.Money{CurrencyCode: currency, Units: n / 100, Nanos: int32(n%100) * 10_000_000}
}
