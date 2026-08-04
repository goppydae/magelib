# Copyright (c) 2026 Steven Verhelle (enqack)
#
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# SPDX-License-Identifier: MPL-2.0

{
  description = "magelib - shared mage build helpers for the goppydae ecosystem";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    # An explicit system list, not eachDefaultSystem: nixpkgs 26.11 dropped
    # x86_64-darwin, which eachDefaultSystem still enumerates, so merely
    # instantiating pkgs for that platform throws. gapi hit this first
    # (GAPI-DIV-035) and goblin followed; this is the same list both carry,
    # so a reader can check all three at a glance. magelib has no package
    # output at all, which makes the darwin dev shell the only thing this
    # list buys here.
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Go toolchain
            go
            gotools # for goimports

            # Build automation
            mage

            # CGO for the race detector
            gcc

            # Protocol Buffers
            protobuf
            buf
            protoc-gen-go
            protoc-gen-go-grpc

            # Lint and security gate
            golangci-lint
            gosec
            govulncheck
          ];

          shellHook = ''
            export GOBIN=$PWD/.bin
            export PATH=$GOBIN:$PATH

            echo "magelib - shared mage build helpers for the goppydae ecosystem"
            echo "Consumed by ../gapi and ../goblin via their committed go.work."
          '';
        };
      }
    );
}
