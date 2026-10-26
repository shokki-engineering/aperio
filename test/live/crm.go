// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package live

// crm.go is the crm repositories, in a file of their own: the repository
// package is named crm, as the service package a test imports is.

import (
	"github.com/shokki-engineering/aperio/internal/database/repository/aperio/crm"
	"github.com/shokki-engineering/aperio/internal/database/repository/repox"
	"github.com/shokki-engineering/aperio/internal/service/generated/aperio/crm/v1/crmpbv1svc"
)

// CrmRepositories are the CrmService's repositories over conn.
func CrmRepositories(conn repox.Conn) crmpbv1svc.CrmServiceRepositories {
	r := crm.New(conn)
	return crmpbv1svc.CrmServiceRepositories{Contacts: r.Contacts, Leads: r.Leads, Activities: r.Activities}
}
