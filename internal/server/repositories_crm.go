// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package server

// repositories_crm.go builds the CrmService's repositories, in a file of its
// own: the repository package is named crm, as the service package
// assembly.go imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/crm"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/crm/v1/crmpbv1svc"
)

// crmRepositories are the CrmService's repositories over conn.
func crmRepositories(conn repox.Conn) crmpbv1svc.CrmServiceRepositories {
	r := crm.New(conn)
	return crmpbv1svc.CrmServiceRepositories{Contacts: r.Contacts, Leads: r.Leads, Activities: r.Activities}
}
