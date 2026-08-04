# Copyright (c) 2026 Steven Verhelle (enqack)
#
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
# SPDX-License-Identifier: MPL-2.0

# Legacy shell.nix for non-flake users
# For flake users, use: nix develop
{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    go
    gotools
    mage
    gcc
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
}
