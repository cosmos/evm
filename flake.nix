{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/release-25.05";
    flake-utils.url = "github:numtide/flake-utils";
    poetry2nix = {
      url = "github:nix-community/poetry2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      poetry2nix,
    }:
    let
      rev = self.shortRev or "dirty";
    in
    (flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = self.overlays.default;
          config = { };
        };
      in
      {
        legacyPackages = pkgs;
        packages.default = pkgs.evmd;
        devShells = {
          default = pkgs.mkShell {
            buildInputs = [
              pkgs.evmd.go
              pkgs.nixfmt-rfc-style
              pkgs.solc
              pkgs.python312
              pkgs.uv
            ];
          };
        };
      }
    ))
    // {
      overlays.default = [
        poetry2nix.overlays.default
        (final: super: {
          evmd = final.callPackage ./. { inherit rev; };
        })
      ];
    };
}
