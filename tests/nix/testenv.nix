{
  poetry2nix,
  lib,
  python3,
}:
poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  python = python3;
  overrides = poetry2nix.overrides.withDefaults (
    self: super:
    let
      buildSystems = {
        pystarport = [ "poetry-core" ];
        cprotobuf = [
          "setuptools"
          "poetry-core"
        ];
        durations = [ "setuptools" ];
        multitail2 = [ "setuptools" ];
        docker = [
          "hatchling"
          "hatch-vcs"
        ];
        flake8-black = [ "setuptools" ];
        flake8-isort = [ "hatchling" ];
        pytest-github-actions-annotate-failures = [ "setuptools" ];
        pyunormalize = [ "setuptools" ];
        typing-inspection = [ "hatchling" ];
        eth-bloom = [ "setuptools" ];
      };
    in
    lib.mapAttrs (
      attr: systems:
      super.${attr}.overridePythonAttrs (old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems;
      })
    ) buildSystems
  );
}
