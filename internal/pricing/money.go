// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package pricing is how aperio prices what it sells, with no I/O: exact
// money arithmetic, GST, which season a date falls in, and which price rule
// applies to an item on a date and what it makes the price.
//
// Amounts are math/big rationals while they are worked on, so no sum or
// percentage loses a paisa to floating point; they are rounded to the
// currency's minor unit (half away from zero) only where a price is stated:
// a unit price, a line's amount, a line's tax.
package pricing

// money.go converts between google.type.Money / google.type.Decimal and
// exact rationals, and rounds.

import (
	"fmt"
	"math/big"

	"google.golang.org/genproto/googleapis/type/decimal"
	"google.golang.org/genproto/googleapis/type/money"
)

// nanos is the number of nanos in a unit.
var nanos = big.NewRat(1_000_000_000, 1)

// zeroDecimal are the ISO 4217 currencies with no minor unit; every other
// is taken to have two, as INR, USD and EUR do.
var zeroDecimal = map[string]bool{"JPY": true, "KRW": true, "VND": true, "CLP": true, "ISK": true, "UGX": true}

// Rat is m's exact value; zero for nil.
func Rat(m *money.Money) *big.Rat {
	r := new(big.Rat).SetInt64(m.GetUnits())
	return r.Add(r, new(big.Rat).Quo(big.NewRat(int64(m.GetNanos()), 1), nanos))
}

// Money is r in currency, rounded to its minor unit.
func Money(r *big.Rat, currency string) *money.Money {
	r = Round(r, currency)
	whole := new(big.Int).Quo(r.Num(), r.Denom()) // truncates toward zero
	frac := new(big.Rat).Sub(r, new(big.Rat).SetInt(whole))
	n := new(big.Rat).Mul(frac, nanos)
	return &money.Money{CurrencyCode: currency, Units: whole.Int64(), Nanos: int32(n.Num().Int64() / n.Denom().Int64())}
}

// Round is r rounded to currency's minor unit, half away from zero.
func Round(r *big.Rat, currency string) *big.Rat {
	scale := big.NewRat(100, 1)
	if zeroDecimal[currency] {
		scale = big.NewRat(1, 1)
	}
	x := new(big.Rat).Mul(r, scale)
	neg := x.Sign() < 0
	x.Abs(x)
	x.Add(x, big.NewRat(1, 2))
	n := new(big.Int).Quo(x.Num(), x.Denom()) // floor, as x >= 0
	if neg {
		n.Neg(n)
	}
	return new(big.Rat).Quo(new(big.Rat).SetInt(n), scale)
}

// Decimal is d's exact value: "18", "-12.5", "1.5e1".
func Decimal(d *decimal.Decimal) (*big.Rat, error) {
	if d.GetValue() == "" {
		return nil, fmt.Errorf("pricing: empty decimal")
	}
	r, ok := new(big.Rat).SetString(d.GetValue())
	if !ok {
		return nil, fmt.Errorf("pricing: %q is not a decimal", d.GetValue())
	}
	return r, nil
}

// Percent is r percent of base: base × r / 100, unrounded.
func Percent(base, r *big.Rat) *big.Rat {
	out := new(big.Rat).Mul(base, r)
	return out.Quo(out, big.NewRat(100, 1))
}

// Mul is r × n, unrounded.
func Mul(r *big.Rat, n int64) *big.Rat {
	return new(big.Rat).Mul(r, big.NewRat(n, 1))
}

// Sum is the sum of rs.
func Sum(rs ...*big.Rat) *big.Rat {
	out := new(big.Rat)
	for _, r := range rs {
		out.Add(out, r)
	}
	return out
}

// SameCurrency fails when m is not in currency: aperio never converts a
// price, it is stated in the venue's currency.
func SameCurrency(m *money.Money, currency, what string) error {
	if m != nil && m.GetCurrencyCode() != "" && m.GetCurrencyCode() != currency {
		return fmt.Errorf("pricing: %s is in %s, not the venue's %s", what, m.GetCurrencyCode(), currency)
	}
	return nil
}
