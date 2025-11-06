{ pkgs }:

let
  nixpkgs-unstable = builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/7241bcbb4f099a66aafca120d37c65e8dda32717.tar.gz";
    sha256 = "1awaf4s01snazyvy7s788m47dzc7k4vd8hj1ikxfvrrgpa1w03k2";
  };
  
  unstable = import nixpkgs-unstable {
    system = pkgs.system;
    config = {};
  };
in
  unstable.go_1_25 or pkgs.go_1_23
