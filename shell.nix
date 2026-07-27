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
    echo "Consumed by ../gapi and ../goblin via replace directives."
  '';
}
