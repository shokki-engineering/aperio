// Copyright 2026 Srikanth K.
// SPDX-License-Identifier: AGPL-3.0-or-later

// Command otagw points the generated protobuf-ota HTTP gateways at the BSR SDK's gRPC
// packages, and gives each gateway a package of its own.
//
// grpc-gateway's standalone mode assumes a service's client and server
// interfaces live in the same Go package as its messages. The BSR SDK for
// buf.build/shokki-engineering/protobuf-ota splits them: messages in
// .../protocolbuffers/go/<path>, gRPC stubs in .../grpc/go/<path>/<pkg>grpc.
// So every standalone gateway names extHotelv1.HotelsClient where the type is
// hotelv1grpc.HotelsClient. This rewrites exactly those references -- the
// <Service>Client, <Service>Server and New<Service>Client of each service the
// file registers -- and adds the import, leaving message references alone.
//
// Standalone mode also declares the gateway in a package named like the
// messages (hotelv1), which no file could import next to them without an
// alias. So each gateway moves into a <pkg>gw sub-package (hotelv1gw), the
// way protoc-gen-mcp's package_suffix puts its bindings in hotelv1mcp.
//
// Usage (run by `just gen ota`, after buf generate):
//
//	go run ./tools/protobuf/otagw [dir]
//
// dir defaults to protobuf/generated/go/ota.
package main

import (
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	messagesModule = "buf.build/gen/go/shokki-engineering/protobuf-ota/protocolbuffers/go"
	grpcModule     = "buf.build/gen/go/shokki-engineering/protobuf-ota/grpc/go"
)

var (
	// sdkImport matches an aliased import of an SDK messages package, e.g.
	// `extHotelv1 "buf.build/.../hotel/hotel/v1"`; the alias is
	// grpc-gateway's choice and not always ext-prefixed.
	sdkImport = regexp.MustCompile(`(?m)^\t(\w+) "` + regexp.QuoteMeta(messagesModule) + `/([^"]+)"$`)
	// services are the services a gateway file registers.
	services = regexp.MustCompile(`func Register(\w+)HandlerServer\(`)
	// packageClause is the file's package declaration.
	packageClause = regexp.MustCompile(`(?m)^package (\w+)$`)
)

func main() {
	dir := "protobuf/generated/go/ota"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".pb.gw.go") {
			files = append(files, path)
		}
		return err
	})
	n := 0
	for _, path := range files {
		if err != nil {
			break
		}
		var changed bool
		if changed, err = rewrite(path); changed {
			n++
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "otagw:", err)
		os.Exit(1)
	}
	fmt.Printf("otagw: pointed %d gateway files at the SDK's gRPC packages\n", n)
}

func rewrite(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	src := string(raw)
	svcs := services.FindAllStringSubmatch(src, -1)
	changed := false
	for _, m := range sdkImport.FindAllStringSubmatch(src, -1) {
		alias, pkgPath := m[1], m[2]
		// The SDK names a package for its last two path elements --
		// .../hotel/hotel/v1 holds hotelv1 -- and puts its gRPC stubs in
		// <that>grpc beneath it.
		parts := strings.Split(pkgPath, "/")
		pkgName := parts[len(parts)-2] + parts[len(parts)-1]
		grpcPkg := pkgName + "grpc"
		used := false
		for _, s := range svcs {
			for _, name := range []string{"New" + s[1] + "Client", s[1] + "Client", s[1] + "Server"} {
				re := regexp.MustCompile(`\b` + alias + `\.` + name + `\b`)
				if re.MatchString(src) {
					src = re.ReplaceAllString(src, grpcPkg+"."+name)
					used = true
				}
			}
		}
		if used {
			grpcPath := grpcModule + "/" + pkgPath + "/" + grpcPkg
			// The path ends in the package's own name, so it imports bare.
			src = strings.Replace(src, m[0], m[0]+"\n\t\""+grpcPath+"\"", 1)
			changed = true
		}
	}
	// Move a gateway still in its messages-named package into <pkg>gw.
	pkg := packageClause.FindStringSubmatch(src)
	target := path
	if pkg != nil && !strings.HasSuffix(pkg[1], "gw") {
		gw := pkg[1] + "gw"
		src = strings.Replace(src, pkg[0], "package "+gw, 1)
		target = filepath.Join(filepath.Dir(path), gw, filepath.Base(path))
		changed = true
	}
	if !changed {
		return false, nil // already rewritten
	}
	out, err := format.Source([]byte(src))
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(target, out, 0o644); err != nil {
		return false, err
	}
	if target != path {
		return true, os.Remove(path)
	}
	return true, nil
}
