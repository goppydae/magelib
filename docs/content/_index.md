---
title: "magelib"
---

# magelib

Shared mage build helpers for the GoPPydae ecosystem. One library holds the
gates that four repositories run, so a rule is written once and cannot be
enforced differently in two places.

`gapi`, `goblin` and `goppydae-docs` each resolve this library through their
committed `go.work` for development and through a published tag for a lone
clone.

## What is here

This site is the generated API reference and nothing else. magelib has no
prose of its own by design: the reasoning behind the build environment lives
in the ecosystem documentation as `design/build-environment.md`, and
duplicating it beside the code would create a second copy to keep in step.

See [the API reference](reference/magelib/) for the exported surface.

## What the library gates

- **Hermeticity** - every tool resolves inside the Nix store, so a host
  install cannot silently satisfy a missing dependency.
- **Shell unification** - sibling dev shells resolve identical store paths
  for the tools they share, and identical versions for the module pins they
  share.
- **Licence headers** - the MPL notice on every hand-written source file, by
  extension rather than by memory.
- **File length** - the 500-line rule, reported per file.
- **Terminology** - retired names cannot come back through a copied comment.
- **Version** - the `VERSION` file agrees with the tag being cut.
- **Documentation** - generated reference matches its source, and no
  document transcribes a value the code defines.

Every gate takes the same `Skip` type, and a skip must state why the rule
does not reach it. An exemption whose reason is not written down is
indistinguishable from an oversight.
