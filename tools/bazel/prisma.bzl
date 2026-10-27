# Copyright 2026 Srikanth K.
# SPDX-License-Identifier: AGPL-3.0-or-later

"""The Prisma CLI for the migrate image (//deploy/image:migrate), fetched by Bazel.

Two repository rules and the module extension MODULE.bazel uses them through:

  cli     node_modules exactly as `npm ci --ignore-scripts` lays it out from a
          committed package-lock.json: every tarball downloaded from the URL
          the lockfile resolved and checked against its integrity hash. No
          npm, no network the lockfile does not name, no install scripts.
  engine  Prisma's schema engine for one Linux platform, the native binary
          `prisma migrate` drives and @prisma/engines' install script would
          otherwise download: fetched from Prisma's binary host by sha256.

The lockfile is written by a pinned npm step (`just prisma-lock`);
its JavaScript is the same on every platform, so one tree serves both
architectures and only the engine differs.
"""

def _cli_impl(rctx):
    lock = json.decode(rctx.read(rctx.attr.lockfile))
    if lock.get("lockfileVersion") != 3:
        fail("%s: want lockfileVersion 3" % rctx.attr.lockfile)

    # The keys are install paths (node_modules/a/node_modules/b), sorted, so
    # a package's directory exists before the packages nested in it.
    for path, pkg in lock["packages"].items():
        if not path:
            continue
        if pkg.get("link") or "os" in pkg or "cpu" in pkg or "libc" in pkg:
            fail("%s: linked and platform-specific packages are not supported" % path)
        tmp = "_extract/" + path
        rctx.download_and_extract(
            url = pkg["resolved"],
            integrity = pkg["integrity"],
            output = tmp,
        )

        # An npm tarball holds one directory, usually package/.
        entries = rctx.path(tmp).readdir()
        if len(entries) != 1:
            fail("%s: the tarball holds %d entries, want one directory" % (path, len(entries)))
        rctx.rename(entries[0], path)
    rctx.delete("_extract")

    rctx.file("BUILD.bazel", """\
load("@rules_pkg//pkg:mappings.bzl", "pkg_attributes", "pkg_files", "strip_prefix")
load("@rules_pkg//pkg:tar.bzl", "pkg_tar")

# node_modules under {dir}, read-only for everyone.
pkg_files(
    name = "node_modules",
    srcs = glob(["node_modules/**"]),
    attributes = pkg_attributes(mode = "0644"),
    prefix = "{dir}",
    strip_prefix = strip_prefix.from_pkg(),
)

pkg_tar(
    name = "layer",
    srcs = [":node_modules"],
    visibility = ["//visibility:public"],
)
""".format(dir = rctx.attr.package_dir))

cli = repository_rule(
    implementation = _cli_impl,
    attrs = {
        "lockfile": attr.label(mandatory = True, allow_single_file = True),
        "package_dir": attr.string(mandatory = True),
    },
    doc = "node_modules from an npm lockfile, as a pkg_tar layer (:layer).",
)

def _engine_impl(rctx):
    rctx.download(
        url = "https://binaries.prisma.sh/all_commits/%s/%s/schema-engine.gz" % (rctx.attr.commit, rctx.attr.platform),
        sha256 = rctx.attr.sha256,
        output = "schema-engine.gz",
    )
    result = rctx.execute(["gzip", "-d", "schema-engine.gz"])
    if result.return_code != 0:
        fail("gzip -d schema-engine.gz: " + result.stderr)
    rctx.file("BUILD.bazel", 'exports_files(["schema-engine"])\n')

engine = repository_rule(
    implementation = _engine_impl,
    attrs = {
        "commit": attr.string(mandatory = True, doc = "The engines commit: @prisma/engines-version's hash."),
        "platform": attr.string(mandatory = True, doc = "Prisma's platform name, e.g. debian-openssl-3.0.x."),
        "sha256": attr.string(mandatory = True, doc = "Of schema-engine.gz."),
    },
    doc = "One platform's Prisma schema engine (:schema-engine).",
)

_cli_tag = tag_class(attrs = {
    "name": attr.string(mandatory = True),
    "lockfile": attr.label(mandatory = True),
    "package_dir": attr.string(mandatory = True),
})

_engine_tag = tag_class(attrs = {
    "name": attr.string(mandatory = True),
    "commit": attr.string(mandatory = True),
    "platform": attr.string(mandatory = True),
    "sha256": attr.string(mandatory = True),
})

def _prisma_impl(mctx):
    for mod in mctx.modules:
        for t in mod.tags.cli:
            cli(name = t.name, lockfile = t.lockfile, package_dir = t.package_dir)
        for t in mod.tags.engine:
            engine(name = t.name, commit = t.commit, platform = t.platform, sha256 = t.sha256)
    return mctx.extension_metadata(reproducible = True)

prisma = module_extension(
    implementation = _prisma_impl,
    tag_classes = {"cli": _cli_tag, "engine": _engine_tag},
    doc = "The Prisma CLI and its schema engines: prisma.cli and prisma.engine.",
)
