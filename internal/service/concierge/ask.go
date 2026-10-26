// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package concierge

// ask.go is the questions the concierge asks instead of failing: the guest's
// details when they are missing, and the guest's yes before anything is
// booked or cancelled -- with the room, the dates and the amount in the
// question, which only the server knows. Each is mcp.Ask's error: a status
// whose ErrorInfo (ELICITATION_REQUIRED) carries the question and the fields
// it asks for. A frontend reads it and shows a form or a confirmation; an MCP
// client is asked it as an elicitation, and the generated tool calls again
// with the answer merged into the request. A field asked for is a field of
// the request, which is how the answer arrives.

import (
	"strings"

	"github.com/shokki-engineering/aperio/internal/service/guest"
	"github.com/the-protobuf-project/runtime-go/agents/mcp"
	"google.golang.org/grpc/codes"
)

// askGuest asks for whichever of the guest's name and phone is missing; nil
// when neither is.
func askGuest(name, phone string) error {
	var fields []mcp.ElicitField
	var missing []string
	if strings.TrimSpace(name) == "" {
		fields = append(fields, mcp.ElicitField{Name: "guest_display_name", Type: "string", Required: true,
			Description: "The guest's name, as the booking is to be under"})
		missing = append(missing, "name")
	}
	if guest.Normalize(phone) == "" {
		fields = append(fields, mcp.ElicitField{Name: "phone", Type: "string", Required: true,
			Description: "The guest's phone number, which finds the booking again"})
		missing = append(missing, "phone number")
	}
	if len(fields) == 0 {
		return nil
	}
	return mcp.Ask(codes.InvalidArgument, "May I have the guest's "+strings.Join(missing, " and ")+"?", fields)
}

// askConsent asks the guest to agree to question; their yes sets the
// request's confirmed.
func askConsent(question string) error {
	return mcp.Ask(codes.FailedPrecondition, question, []mcp.ElicitField{{
		Name: "confirmed", Type: "boolean", Required: true, Description: "Yes, go ahead",
	}})
}
