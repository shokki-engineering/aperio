// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package operations

// states.go is the custom methods that move a service request through its
// states:
//
//	OPEN -> ACKNOWLEDGED -> IN_PROGRESS -> DONE
//
// AcknowledgeServiceRequest takes up an OPEN request (acknowledge_time);
// StartServiceRequest sets an ACKNOWLEDGED one on its way;
// CompleteServiceRequest marks an ACKNOWLEDGED or IN_PROGRESS one DONE
// (complete_time) -- a pot of coffee is often taken up and carried in one
// go -- posting its charge to its booking first (charge.go) and then telling
// the guest (notify.go); CancelServiceRequest calls off any request not yet
// DONE, the reason kept in its notes. DONE and CANCELLED are final.

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/shokki-engineering/aperio/internal/booking"
	"github.com/shokki-engineering/aperio/internal/service/generated/servicex"
	"github.com/shokki-engineering/aperio/protobuf/generated/go/operations/v1/operationspbv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// requestType is the resource type errors name.
const requestType = "aperio.operations.v1/ServiceRequest"

// maxNotes is the most a request's notes hold (the proto's max_len).
const maxNotes = 2000

// outputOnly are the server-owned fields a state method writes.
var outputOnly = []string{"state", "acknowledge_time", "complete_time"}

// The states, by their short Go names.
const (
	stateOpen         = operationspbv1.ServiceRequest_STATE_OPEN
	stateAcknowledged = operationspbv1.ServiceRequest_STATE_ACKNOWLEDGED
	stateInProgress   = operationspbv1.ServiceRequest_STATE_IN_PROGRESS
	stateDone         = operationspbv1.ServiceRequest_STATE_DONE
	stateCancelled    = operationspbv1.ServiceRequest_STATE_CANCELLED
)

// AcknowledgeServiceRequest takes up an open request for its department.
func (s *Server) AcknowledgeServiceRequest(ctx context.Context, req *operationspbv1.AcknowledgeServiceRequestRequest) (*operationspbv1.ServiceRequest, error) {
	return s.move(ctx, "AcknowledgeServiceRequest", req.GetName(), req.GetEtag(), []operationspbv1.ServiceRequest_State{stateOpen},
		func(r *operationspbv1.ServiceRequest) ([]string, error) {
			r.State, r.AcknowledgeTime = stateAcknowledged, timestamppb.New(s.h.now())
			return []string{"state", "acknowledge_time"}, nil
		})
}

// StartServiceRequest marks an acknowledged request as on its way.
func (s *Server) StartServiceRequest(ctx context.Context, req *operationspbv1.StartServiceRequestRequest) (*operationspbv1.ServiceRequest, error) {
	return s.move(ctx, "StartServiceRequest", req.GetName(), req.GetEtag(), []operationspbv1.ServiceRequest_State{stateAcknowledged},
		func(r *operationspbv1.ServiceRequest) ([]string, error) {
			r.State = stateInProgress
			return []string{"state"}, nil
		})
}

// CompleteServiceRequest marks a request done: its charge is posted to its
// booking first, so a request is never DONE with its charge lost, and the
// guest is told after.
func (s *Server) CompleteServiceRequest(ctx context.Context, req *operationspbv1.CompleteServiceRequestRequest) (*operationspbv1.ServiceRequest, error) {
	out, err := s.move(ctx, "CompleteServiceRequest", req.GetName(), req.GetEtag(), []operationspbv1.ServiceRequest_State{stateAcknowledged, stateInProgress},
		func(r *operationspbv1.ServiceRequest) ([]string, error) {
			if err := s.h.charge(ctx, r); err != nil {
				return nil, err
			}
			r.State, r.CompleteTime = stateDone, timestamppb.New(s.h.now())
			return []string{"state", "complete_time"}, nil
		})
	if err == nil {
		s.h.tellRequester(ctx, out)
	}
	return out, err
}

// CancelServiceRequest calls off a request not yet done; the reason is added
// to its notes.
func (s *Server) CancelServiceRequest(ctx context.Context, req *operationspbv1.CancelServiceRequestRequest) (*operationspbv1.ServiceRequest, error) {
	return s.move(ctx, "CancelServiceRequest", req.GetName(), req.GetEtag(), []operationspbv1.ServiceRequest_State{stateOpen, stateAcknowledged, stateInProgress},
		func(r *operationspbv1.ServiceRequest) ([]string, error) {
			r.State = stateCancelled
			if reason := strings.TrimSpace(req.GetReason()); reason != "" {
				r.Notes = clip(strings.TrimSpace(r.GetNotes()+"\nCancelled: "+reason), maxNotes)
				return []string{"state", "notes"}, nil
			}
			return []string{"state"}, nil
		})
}

// move runs one state method: it reads the request, checks the caller's etag
// and that it is in one of from, applies change, and writes the fields
// change names, under the etag it read.
func (s *Server) move(ctx context.Context, method, name, etag string, from []operationspbv1.ServiceRequest_State,
	change func(*operationspbv1.ServiceRequest) ([]string, error)) (*operationspbv1.ServiceRequest, error) {
	var out *operationspbv1.ServiceRequest
	err := s.Options().Run(ctx, "OperationsService", method, func(ctx context.Context) error {
		r, err := s.Repositories().ServiceRequests.Get(ctx, name)
		if err != nil {
			return servicex.NotFoundAs(err, requestType, name)
		}
		if err := servicex.CheckEtag(requestType, name, etag, r.GetEtag()); err != nil {
			return err
		}
		if !booking.InState(r.GetState(), from) {
			return booking.WrongState(requestType, name, method, r.GetState().String())
		}
		paths, err := change(r)
		if err != nil {
			return err
		}
		server := slices.DeleteFunc(slices.Clone(paths), func(p string) bool { return !slices.Contains(outputOnly, p) })
		if len(server) == len(paths) {
			out, err = s.Repositories().ServiceRequests.UpdateOutputOnly(ctx, r, paths)
		} else {
			own := slices.DeleteFunc(paths, func(p string) bool { return slices.Contains(outputOnly, p) })
			out, err = s.Repositories().ServiceRequests.UpdateWithOutputOnly(ctx, r, own, server)
		}
		return servicex.NotFoundAs(err, requestType, name)
	})
	return out, err
}

// clip is s cut to at most n runes, keeping its end: the newest note.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[len(r)-n:])
}
