{
  lib,
  stdenv,
  buildGo123Module,
  fetchFromGitHub,
  rev ? "dirty",
  nativeByteOrder ? true, # nativeByteOrder mode will panic on big endian machines
  fetchurl,
  pkgsStatic,
}:
let
  version = "v0.5.0";
  pname = "evmd";

  # Use static packages for Linux to ensure musl compatibility
  buildPackages = if stdenv.isLinux then pkgsStatic else { inherit stdenv buildGo123Module; };
  buildStdenv = buildPackages.stdenv;
  buildGo123Module' = if stdenv.isLinux then buildPackages.buildGo123Module else buildGo123Module;

  tags =
    [
      "ledger"
      "netgo"
      "osusergo"
      "pebbledb"
    ]
    ++ lib.optionals nativeByteOrder [ "nativebyteorder" ]
    ++ lib.optionals buildStdenv.isLinux [ "muslc" ];

  ldflags =
    [
      "-X github.com/cosmos/cosmos-sdk/version.Name=evmd"
      "-X github.com/cosmos/cosmos-sdk/version.AppName=${pname}"
      "-X github.com/cosmos/cosmos-sdk/version.Version=${version}"
      "-X github.com/cosmos/cosmos-sdk/version.BuildTags=${lib.concatStringsSep "," tags}"
      "-X github.com/cosmos/cosmos-sdk/version.Commit=${rev}"
    ]
    ++ [
      "-w"
      "-s"
      "-linkmode=external"
    ]
    ++ lib.optionals buildStdenv.isLinux [
      "-extldflags '-static -lm'"
    ];

in
buildGo123Module' rec {
  inherit
    pname
    version
    tags
    ldflags
    ;
  stdenv = buildStdenv;
  src = fetchFromGitHub {
    owner = "cosmos";
    repo = "evm";
    rev = "733ea6190eb1be15ed1276ab54f4a8b511ff1c66";
    hash = "sha256-mZQhW6CpIUGPj3WWw5v2Zeov1mMnKx+ZqNX3YcFOcVE=";
  };
  
  vendorHash = "sha256-IDJHj2e2LBMe0BtwduG7/wLM/C2rRQyIUpbMawJAilk=";
  proxyVendor = true;
  sourceRoot = "source/evmd";
  subPackages = [ "cmd/evmd" ];
  CGO_ENABLED = "1";

  preBuild = ''
    mkdir -p $TMPDIR/lib
    export CGO_LDFLAGS="-L$TMPDIR/lib $CGO_LDFLAGS"
  '';

  doCheck = false;
  meta = with lib; {
    description = "An EVM compatible framework for blockchain development with the Cosmos SDK";
    homepage = "https://github.com/cosmos/evm";
    license = licenses.asl20;
    mainProgram = "evmd" + buildStdenv.hostPlatform.extensions.executable;
    platforms = platforms.all;
  };
}