# magelib

Shared mage build helpers for the goppydae ecosystem. This repo is the build
vocabulary factored once and imported by every sibling magefile: the hermeticity
guard, the doctor checks, formatting, linting, proto generation, and version
resolution.

## Position in the silo

This repo is a sibling of the two code repos and is consumed through a local
`replace` directive, the same mechanism that couples the orchestrator to the
kernel:

```
workspace/
  gapi/     replace github.com/goppydae/magelib => ../magelib
  goblin/   replace github.com/goppydae/gapi    => ../gapi
            replace github.com/goppydae/magelib => ../magelib
  magelib/  (this repo)
```

The sibling layout is load-bearing: a lone clone of a consumer repo does not
build its magefile. The extraction of these helpers out of the kernel repo was
triggered by the written rule in the ecosystem manifesto (section 7): the
helpers move to their own repo when a third repo joins the ecosystem or when a
tagged release needs build helpers pinned independently of the kernel cadence.

## Layout

- `pkg/magelib/` - the helper library imported by consumer magefiles.
- `.golangci.yml` - the pinned linter configuration consumed by every repo's
  `Lint` target.
- `divergence.jsonl` / `deprecation.jsonl` - this repo's ledgers, seeded empty.

## Working here

Enter the dev shell before anything else:

```
nix develop
```

Consumers vendor this module; after changing helper code, re-run
`go mod vendor` in each consumer and verify both build green. The orchestrator
build is the contract check for kernel changes; both consumer builds are the
contract check for changes here.

## Directory conventions

`GOBIN=$PWD/.bin` is the tool-install directory inside the dev shell; product
builds in consumer repos write to their own `bin/`. The two directories have
distinct roles and are both gitignored.
