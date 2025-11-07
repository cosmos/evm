{
  src,
  lib,
  stdenv,
  darwin,
  rustPackages_1_83,
  symlinkJoin,
  openssl,
  pkg-config,
  protobuf,
  clang,
  llvmPackages,
}:

rustPackages_1_83.rustPlatform.buildRustPackage rec {
  name = "hermes";
  inherit src;
  cargoBuildFlags = [ "-p" "ibc-relayer-cli" ];
  nativeBuildInputs = [
    pkg-config
    protobuf
    clang
  ];
  buildInputs = [
    openssl
    llvmPackages.libclang.lib
  ] ++ lib.optionals stdenv.isDarwin [
    darwin.apple_sdk.frameworks.Security
    darwin.libiconv
    darwin.apple_sdk.frameworks.SystemConfiguration
  ];
  cargoLock = {
    lockFile = "${src}/Cargo.lock";
  };
  doCheck = false;
  env = {
    RUSTFLAGS = "--cfg ossl111 --cfg ossl110 --cfg ossl101";
    OPENSSL_NO_VENDOR = "1";
    PROTOC = "${protobuf}/bin/protoc";
    LIBCLANG_PATH = "${llvmPackages.libclang.lib}/lib";
    OPENSSL_DIR = symlinkJoin {
      name = "openssl";
      paths = with openssl; [
        out
        dev
      ];
    };
  };
}