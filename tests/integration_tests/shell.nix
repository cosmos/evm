{
  system ? builtins.currentSystem,
  pkgs ? import ../nix { inherit system; },
}:
pkgs.mkShell {
  buildInputs = [
    pkgs.git
    pkgs.test-env
    pkgs.poetry
    pkgs.solc
    # pkgs.evmd
  ];
  shellHook = ''
    export TMPDIR=/tmp
  '';
}
