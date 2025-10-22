{
  sources ? import ./sources.nix,
  system ? builtins.currentSystem,
  ...
}:
import sources.nixpkgs {
  overlays = [
    (_: pkgs: {
      flake-compat = import sources.flake-compat;
      dapp = pkgs.dapp;
    })
    (import "${sources.poetry2nix}/overlay.nix")
    (_: pkgs: { test-env = pkgs.callPackage ./testenv.nix { }; })
    (_: pkgs: { evmd = pkgs.callPackage ./evm/default.nix { }; })
  ];
  config = { };
  inherit system;
}
