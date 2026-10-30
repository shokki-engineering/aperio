// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// policy_table.go is who may call each RPC of each service, besides the
// owner, who may call all of them. Reads and writes are split where they
// differ: kitchen reads sessions, menus and the day's dietary needs and
// works service requests, and sees no invoice; accounts runs finance and
// reads the rest; the concierge is public. A new RPC needs a line here, or
// TestEveryMethodHasAPolicy fails and the method is refused to all but no
// one. Sharing a guest's link (Share*) is the staff's who own the resource:
// sales a quote, the front desk a reservation, accounts an invoice.

// Role sets the table uses.
var (
	desk     = []Role{FrontDesk}
	selling  = []Role{Sales}
	money    = []Role{Accounts}
	floor    = []Role{FrontDesk, Kitchen, Housekeeping}
	contacts = []Role{FrontDesk, Sales, Accounts}
	pricers  = []Role{FrontDesk, Sales, Accounts}
	menus    = []Role{FrontDesk, Sales, Kitchen, Accounts}
	staff    = []Role{FrontDesk, Sales, Kitchen, Housekeeping, Accounts}
	stays    = []Role{FrontDesk, Sales, Accounts}
	day      = []Role{FrontDesk, Sales, Kitchen, Housekeeping, Accounts}
	events   = []Role{FrontDesk, Sales, Kitchen, Accounts}
	leads    = []Role{FrontDesk, Sales}
	quotes   = []Role{FrontDesk, Sales, Accounts}
	pipeline = []Role{Sales, Accounts}
	bills    = []Role{FrontDesk, Accounts}
	requests = []Role{FrontDesk, Kitchen, Housekeeping, Accounts}
	owner    []Role // the owner alone
)

// allow is a rule for roles; global, link and unlimited add the flags.
func allow(roles []Role) Rule  { return Rule{Roles: roles} }
func global(roles []Role) Rule { return Rule{Roles: roles, Global: true} }
func link(roles []Role) Rule   { return Rule{Roles: roles, Link: true} }

// public is the rule of the guest-facing concierge.
var public = Rule{Public: true}

// policyTable is the rule for each method, by service then method.
var policyTable = map[string]map[string]Rule{
	"aperio.venue.v1.VenueService": {
		"GetVenue": allow(staff), "ListVenues": global(staff),
		"CreateVenue": {Global: true, Unlimited: true}, "UpdateVenue": allow(owner),
		"GetSpace": allow(staff), "ListSpaces": allow(staff),
		"CreateSpace": allow(owner), "UpdateSpace": allow(owner),
	},
	"aperio.catalog.v1.CatalogService": {
		"GetDish": allow(menus), "ListDishes": allow(menus),
		"CreateDish": allow(owner), "UpdateDish": allow(owner),
		"GetMenu": allow(menus), "ListMenus": allow(menus),
		"CreateMenu": allow(owner), "UpdateMenu": allow(owner),
		"GetFixedPackage": allow(pricers), "ListFixedPackages": allow(pricers),
		"CreateFixedPackage": allow(owner), "UpdateFixedPackage": allow(owner),
	},
	"aperio.pricing.v1.PricingService": {
		"GetSeason": allow(pricers), "ListSeasons": allow(pricers),
		"CreateSeason": allow(owner), "UpdateSeason": allow(owner), "DeleteSeason": allow(owner),
		"GetPriceRule": allow(pricers), "ListPriceRules": allow(pricers),
		"CreatePriceRule": allow(owner), "UpdatePriceRule": allow(owner), "DeletePriceRule": allow(owner),
		"CalculatePrice": allow(pricers),
	},
	"aperio.guest.v1.GuestService": {
		"GetContact": global(contacts), "ListContacts": global(contacts),
		"CreateContact": global(leads), "UpdateContact": global(leads),
		"SummarizeContact": global(contacts),
	},
	"aperio.sales.v1.SalesService": {
		"GetLead": global(leads), "ListLeads": global(leads),
		"CreateLead": global(leads), "UpdateLead": global(leads),
		"GetActivity": global(leads), "ListActivities": global(leads), "CreateActivity": global(leads),
		"GetQuote": link(quotes), "ListQuotes": allow(quotes),
		"CreateQuote": allow(selling), "UpdateQuote": allow(selling), "SendQuote": allow(selling),
		"AcceptQuote": allow(selling), "DeclineQuote": allow(selling), "ReviseQuote": allow(selling),
		"SummarizePipeline": allow(pipeline), "ShareQuote": allow(selling),
	},
	"aperio.stay.v1.StayService": {
		"GetReservation": link(stays), "ListReservations": allow(stays),
		"CreateReservation": allow(desk), "UpdateReservation": allow(desk),
		"ConfirmReservation": allow(desk), "CancelReservation": allow(desk),
		"ArriveReservation": allow(desk), "DepartReservation": allow(desk),
		"SummarizeDay": allow(day), "ShareReservation": allow(desk),
	},
	"aperio.event.v1.EventService": {
		"GetEvent": allow(events), "ListEvents": allow(events),
		"CreateEvent": allow(selling), "UpdateEvent": allow(selling),
		"ConfirmEvent": allow(selling), "CancelEvent": allow(selling),
		"GetSession": allow(events), "ListSessions": allow(events),
		"CreateSession": allow(selling), "UpdateSession": allow(selling), "DeleteSession": allow(selling),
		"SummarizeSessions": allow(events),
	},
	"aperio.finance.v1.FinanceService": {
		"GetInvoice": link(bills), "ListInvoices": allow(bills),
		"CreateInvoice": allow(money), "UpdateInvoice": allow(money), "IssueInvoice": allow(money),
		"CancelInvoice": allow(money), "CreditInvoice": allow(money),
		"GetPayment": allow(bills), "ListPayments": allow(bills),
		"CreatePayment": allow(bills), "UpdatePayment": allow(money), "RefundPayment": allow(money),
		"VoidPayment": allow(money), "ClearPayment": allow(money), "RequestPayment": allow(bills),
		"SummarizeReceivables": allow(money), "ShareInvoice": allow(money),
	},
	"aperio.document.v1.DocumentService": {
		"GetDocument": allow(money), "ListDocuments": allow(money),
		"CreateDocument": allow(money), "UpdateDocument": allow(money), "DeleteDocument": allow(owner),
	},
	"aperio.operations.v1.OperationsService": {
		"GetServiceRequest": allow(requests), "ListServiceRequests": allow(requests),
		"CreateServiceRequest": allow(floor), "AcknowledgeServiceRequest": allow(floor),
		"StartServiceRequest": allow(floor), "CompleteServiceRequest": allow(floor),
		"CancelServiceRequest": allow(floor), "SummarizeServiceRequests": allow(requests),
	},
	"aperio.concierge.v1.ConciergeService": {
		"FindStay": public, "HoldStay": public, "ConfirmStay": public, "FindBooking": public,
		"CancelBooking": public, "EnquireEvent": public, "RequestService": public,
	},
}
