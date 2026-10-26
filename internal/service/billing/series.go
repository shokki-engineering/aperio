// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package billing

// series.go numbers invoices. GST (CGST Rules, rule 46(b)) asks for a
// consecutive serial number, unique for the financial year, of at most 16
// characters. aperio keeps one series per venue, kind and financial year
// (April to March):
//
//	tax invoice      SBR/2627/0042
//	credit note      SBR/CN2627/007   (rule 53: a series of its own)
//	proforma         SBR/PF2627/015
//	receipt voucher  SBR/RV2627/003
//
// The next number is the highest issued in the series plus one; the unique
// index on number makes two issues racing for it fail one of them, which
// IssueInvoice retries (states.go).

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/finance/v1/financepbv1"
)

// maxNumber is the longest invoice number GST allows.
const maxNumber = 16

// kindCodes mark each kind's series; a tax invoice's has none.
var kindCodes = map[financepbv1.InvoiceKind]string{
	financepbv1.InvoiceKind_INVOICE_KIND_TAX_INVOICE:     "",
	financepbv1.InvoiceKind_INVOICE_KIND_CREDIT_NOTE:     "CN",
	financepbv1.InvoiceKind_INVOICE_KIND_PROFORMA:        "PF",
	financepbv1.InvoiceKind_INVOICE_KIND_RECEIPT_VOUCHER: "RV",
}

// series is the stem of the series kind is numbered in for a document
// issued on day, e.g. "SBR/2627/" or "SBR/CN2627/".
func series(prefix string, kind financepbv1.InvoiceKind, day time.Time) (string, error) {
	code, ok := kindCodes[kind]
	if !ok {
		return "", fmt.Errorf("billing: %v has no series", kind)
	}
	y := day.Year()
	if day.Month() < time.April {
		y--
	}
	return fmt.Sprintf("%s/%s%02d%02d/", prefix, code, y%100, (y+1)%100), nil
}

// number is the next number in stem at venueName: four digits at least,
// more as the series grows, and never past 16 characters.
func (h *hooks) number(ctx context.Context, venueName, stem string, kind financepbv1.InvoiceKind) (string, error) {
	last, err := h.last(ctx, venueName, stem, kind)
	if err != nil {
		return "", err
	}
	n := fmt.Sprintf("%s%04d", stem, last+1)
	if len(n) > maxNumber {
		return "", fmt.Errorf("billing: series %s is full: %s is longer than %d characters", stem, n, maxNumber)
	}
	return n, nil
}

// last is the highest serial issued in stem, 0 when none has been. Numbers
// are read newest first, so the walk ends at the first one below the stem.
func (h *hooks) last(ctx context.Context, venueName, stem string, kind financepbv1.InvoiceKind) (int, error) {
	in := repox.ListInput{PageSize: 100, OrderBy: "number desc", Filter: "kind = " + kind.String()}
	best := 0
	for {
		page, next, err := h.repos.Invoices.List(ctx, venueName, in)
		if err != nil {
			return 0, err
		}
		for _, inv := range page {
			num := inv.GetNumber()
			if num == "" {
				continue // a draft: not yet numbered
			}
			if !strings.HasPrefix(num, stem) {
				if num < stem {
					return best, nil
				}
				continue
			}
			if n, err := strconv.Atoi(strings.TrimPrefix(num, stem)); err == nil && n > best {
				best = n
			}
		}
		if next == "" {
			return best, nil
		}
		in.PageToken = next
	}
}
