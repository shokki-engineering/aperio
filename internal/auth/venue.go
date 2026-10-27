// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

// venue.go keeps a venue-limited principal to its venues. The venues a
// request touches are read off it by reflection: every name, parent and
// venue field, and every field referring to a resource
// ((google.api.resource_reference)), at the top and in aperio's own nested
// messages, whose value is under venues/{venue}. Each must be one the
// principal reaches. "venues/-" -- every venue, AIP-159 -- is refused to a
// limited principal: narrowing it would mean rewriting each List's parent
// and filter, so it is asked to list venue by venue instead. A request
// naming no venue is refused too, unless its method is Global (contacts,
// leads, the venue list), whose responses are narrowed instead.

import (
	"strings"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// depth is how deep venues are looked for: a request, the resource it
// carries, and that resource's own parts.
const depth = 3

// Venues are the venue names (venues/{venue}, or venues/- for every venue)
// m refers to.
func Venues(m proto.Message) []string {
	var out []string
	collect(m.ProtoReflect(), depth, &out)
	return out
}

// collect adds the venues m's fields refer to, looking levels deep.
func collect(m protoreflect.Message, levels int, out *[]string) {
	if levels == 0 || !m.IsValid() {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.Kind() == protoreflect.StringKind && refers(fd):
			if fd.IsList() {
				for i := range v.List().Len() {
					add(v.List().Get(i).String(), out)
				}
			} else if !fd.IsMap() {
				add(v.String(), out)
			}
		case fd.Kind() == protoreflect.MessageKind && own(fd.Message()):
			if fd.IsList() {
				for i := range v.List().Len() {
					collect(v.List().Get(i).Message(), levels-1, out)
				}
			} else if !fd.IsMap() {
				collect(v.Message(), levels-1, out)
			}
		}
		return true
	})
}

// refers reports whether fd names a resource: a name, parent or venue
// field, or one annotated as a resource reference.
func refers(fd protoreflect.FieldDescriptor) bool {
	switch fd.Name() {
	case "name", "parent", "venue":
		return true
	}
	return proto.HasExtension(fd.Options(), annotations.E_ResourceReference)
}

// own reports whether md is one of aperio's messages, whose venue
// references are looked into; OpenTravel's and Google's are not.
func own(md protoreflect.MessageDescriptor) bool {
	return strings.HasPrefix(string(md.FullName()), "aperio.")
}

// add appends the venue name s is under, when it is under one.
func add(s string, out *[]string) {
	rest, ok := strings.CutPrefix(s, "venues/")
	if !ok || rest == "" {
		return
	}
	id, _, _ := strings.Cut(rest, "/")
	*out = append(*out, "venues/"+id)
}

// reachesAll reports whether p reaches every venue in venues, and which one
// it does not when it does not.
func reachesAll(p Principal, venues []string) (string, bool) {
	for _, v := range venues {
		if v == "venues/-" && !p.Unlimited() || !p.Reaches(v) {
			return v, false
		}
	}
	return "", true
}

// narrow is resp as a venue-limited p may see it: each item of a repeated
// field of aperio messages that refers to a venue p does not reach is
// dropped; any other field that does makes the whole response refused
// (false).
func narrow(p Principal, resp proto.Message) bool {
	m := resp.ProtoReflect()
	allowed := true
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsList() && fd.Kind() == protoreflect.MessageKind && own(fd.Message()) {
			list, kept := v.List(), 0
			for i := range list.Len() {
				var venues []string
				collect(list.Get(i).Message(), depth-1, &venues)
				if _, ok := reachesAll(p, venues); ok {
					list.Set(kept, list.Get(i))
					kept++
				}
			}
			list.Truncate(kept)
			return true
		}
		var venues []string
		collect(single(m, fd), depth, &venues)
		if _, ok := reachesAll(p, venues); !ok {
			allowed = false
			return false
		}
		return true
	})
	return allowed
}

// single is a message holding fd's value of m alone, so collect judges one
// field at a time.
func single(m protoreflect.Message, fd protoreflect.FieldDescriptor) protoreflect.Message {
	one := m.New()
	one.Set(fd, m.Get(fd))
	return one
}
