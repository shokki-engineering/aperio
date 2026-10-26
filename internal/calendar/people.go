// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

// people.go reads who a booking is for: the guests' names out of OpenTravel
// ResGuests, and the contact's name, emails, phones and address. The
// OpenTravel types are read by their field names through protoreflect, so
// this file needs none of protobuf-ota's many `types` packages.

import (
	"fmt"
	"strings"

	"github.com/shokki-engineering/aperio/protobuf/generated/go/crm/v1/crmpbv1"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/type/v1/typepbv1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// guestNames are the names of every OpenTravel PersonName in ms, as
// "Given Middle Surname", without repeats.
func guestNames[M proto.Message](ms []M) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range ms {
		walk(m.ProtoReflect(), func(pm protoreflect.Message) {
			if pm.Descriptor().Name() != "PersonName" {
				return
			}
			n := strings.Join(append(append(texts(pm, "given_names"), texts(pm, "middle_names")...), texts(pm, "surname")...), " ")
			if n != "" && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		})
	}
	return out
}

// contactLines describes c: its name, emails, phones and first address.
func contactLines(c *crmpbv1.Contact) []string {
	if c == nil {
		return nil
	}
	out := []string{"Contact: " + c.GetDisplayName()}
	if len(c.GetEmails()) > 0 {
		out = append(out, "Email: "+strings.Join(c.GetEmails(), ", "))
	}
	var phones []string
	for _, p := range c.GetPhones() {
		phones = append(phones, p.GetNumber())
	}
	if len(phones) > 0 {
		out = append(out, "Phone: "+strings.Join(phones, ", "))
	}
	for _, a := range c.GetAddresses() {
		pm := a.ProtoReflect()
		parts := append(texts(pm, "address_lines"), append(texts(pm, "city_display_name"), texts(pm, "postal_code")...)...)
		if len(parts) > 0 {
			out = append(out, "Address: "+strings.Join(parts, ", "))
			break
		}
	}
	return out
}

// dietLines describes dietary requirements: "Diet: 12 × JAIN (no nuts)".
func dietLines(ds []*typepbv1.DietaryRequirement) []string {
	var out []string
	for _, d := range ds {
		s := fmt.Sprintf("Diet: %d × %s", max(d.GetGuests(), 1), strings.TrimPrefix(d.GetDiet().String(), "DIET_"))
		if d.GetGuestDisplayName() != "" {
			s += " for " + d.GetGuestDisplayName()
		}
		if d.GetNotes() != "" {
			s += " (" + d.GetNotes() + ")"
		}
		out = append(out, s)
	}
	return out
}

// walk calls fn on m and every message set within it.
func walk(m protoreflect.Message, fn func(protoreflect.Message)) {
	fn(m)
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Message() == nil || fd.IsMap():
		case fd.IsList():
			for i := 0; i < v.List().Len(); i++ {
				walk(v.List().Get(i).Message(), fn)
			}
		default:
			walk(v.Message(), fn)
		}
		return true
	})
}

// texts are the non-empty values of m's string field name, single or
// repeated; none when m has no such field.
func texts(m protoreflect.Message, name protoreflect.Name) []string {
	fd := m.Descriptor().Fields().ByName(name)
	if fd == nil || fd.Kind() != protoreflect.StringKind || !m.Has(fd) {
		return nil
	}
	if !fd.IsList() {
		return []string{m.Get(fd).String()}
	}
	var out []string
	for l, i := m.Get(fd).List(), 0; i < l.Len(); i++ {
		if s := l.Get(i).String(); s != "" {
			out = append(out, s)
		}
	}
	return out
}
