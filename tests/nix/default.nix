{
  sources ? import ./sources.nix,
  system ? builtins.currentSystem,
  ...
}:
import sources.nixpkgs {
  overlays = [
    (_: pkgs: {
      flake-compat = import sources.flake-compat;
    })
    (import "${sources.poetry2nix}/overlay.nix")
    (_: pkgs: { test-env = pkgs.callPackage ./testenv.nix { }; })
    (_: pkgs: { evmd = pkgs.callPackage ./evm/default.nix { }; })
    (_: pkgs: { 
      go_1_25 = pkgs.callPackage ./go_1_25.nix { };
      evmd = pkgs.callPackage ./evm/default.nix { }; 
    })
  ];
  config = { };
  inherit system;
}
