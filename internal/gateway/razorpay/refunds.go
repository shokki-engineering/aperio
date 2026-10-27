// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package razorpay

// refunds.go refunds a payment (POST /payments/{id}/refund). A payment made
// on a link is recorded under the link's id (plink_...), which its webhook
// names; the refund needs the payment's own (pay_...), read from the link.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/genproto/googleapis/type/money"
)

// refundBody is a refund to make.
type refundBody struct {
	Amount int64             `json:"amount"`
	Speed  string            `json:"speed"`
	Notes  map[string]string `json:"notes"`
}

// Refund returns amount of the payment, or the link's payment, Razorpay
// knows as reference, and gives the refund's id, which its webhook names.
func (c *Client) Refund(ctx context.Context, reference string, amount *money.Money) (string, error) {
	payment := reference
	if strings.HasPrefix(reference, "plink_") {
		var l link
		if err := c.do(ctx, http.MethodGet, "/payment_links/"+url.PathEscape(reference), nil, &l); err != nil {
			return "", err
		}
		payment = ""
		for _, p := range l.Payments {
			if p.Status == "captured" {
				payment = p.PaymentID
			}
		}
		if payment == "" {
			return "", fmt.Errorf("razorpay: link %s has no captured payment to refund", reference)
		}
	}
	var out struct {
		ID string `json:"id"`
	}
	body := refundBody{Amount: paise(amount), Speed: "normal", Notes: map[string]string{noteKey: reference}}
	if err := c.do(ctx, http.MethodPost, "/payments/"+url.PathEscape(payment)+"/refund", body, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}
