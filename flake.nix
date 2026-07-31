{
  description = "magelib - shared mage build helpers for the goppydae ecosystem";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
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
