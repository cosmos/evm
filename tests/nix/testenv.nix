{
  poetry2nix,
  lib,
  python3,
  rustc,
  cargo,
  maturin,
  cacert,
  openssl,
  pkg-config,
}:
poetry2nix.mkPoetryEnv {
  projectDir = ../integration_tests;
  python = python3;
  extraPackages = ps: [ ps.setuptools-rust ];
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
        cryptography = [ "setuptools" "setuptools-rust" ];
        pycryptodome = [ "setuptools" "setuptools-rust" ];
        bcrypt = [ "setuptools" "setuptools-rust" ];
        lxml = [ "setuptools" "setuptools-rust" ];
        pyrevm = [ "setuptools" "setuptools-rust" ];
      };
      packageOverrides = {
        maturin = super.maturin.override { preferWheel = true; };
        pyrevm = super.pyrevm.overridePythonAttrs (old: {
          nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [
            maturin
            rustc
            cargo
            pkg-config
            self.setuptools-rust
          ];
          buildInputs = (old.buildInputs or [ ]) ++ [
            openssl
          ];
          preBuild = ''
            export SSL_CERT_FILE="${cacert}/etc/ssl/certs/ca-bundle.crt"
            export CARGO_NET_GIT_FETCH_WITH_CLI=true
            export CARGO_HTTP_CAINFO="${cacert}/etc/ssl/certs/ca-bundle.crt"
            export REQUESTS_CA_BUNDLE="${cacert}/etc/ssl/certs/ca-bundle.crt"
            export CURL_CA_BUNDLE="${cacert}/etc/ssl/certs/ca-bundle.crt"
            
            # Set up writable home directory for cargo
            export HOME=$(mktemp -d)
            mkdir -p $HOME/.cargo
            
            # Create cargo config to use git cli and set proper registry
            cat > $HOME/.cargo/config.toml << EOF
            [net]
            git-fetch-with-cli = true

            [http]
            cainfo = "${cacert}/etc/ssl/certs/ca-bundle.crt"

            [registry]
            default = "crates-io"

            [registries.crates-io]
            index = "https://github.com/rust-lang/crates.io-index"
            EOF
          '';
        });
      };
    in
    (lib.mapAttrs (
      attr: systems:
      super.${attr}.overridePythonAttrs (old: {
        nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ map (a: self.${a}) systems
          ++ lib.optionals (builtins.elem "setuptools-rust" systems) [
            rustc
            cargo
          ];
      })
    ) buildSystems) // packageOverrides
  );
}
