{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/release-25.05";
    flake-utils.url = "github:numtide/flake-utils";
    poetry2nix = {
      url = "github:nix-community/poetry2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
    hermes = {
      url = "github:mmsqe/ibc-rs/ae80ab348952840696e6c9a0c7096d2de11ea579";
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      poetry2nix,
      hermes,
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
              pkgs.evmd
              pkgs.nixfmt-rfc-style
              pkgs.solc
              pkgs.python312
              pkgs.direnv
              pkgs.uv
              pkgs.git
              pkgs.hermes
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
          hermes = final.callPackage ./tests/nix/hermes.nix { src = hermes; };
        })
      ];
    };
}
