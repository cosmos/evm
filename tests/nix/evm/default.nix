{
  lib,
  stdenv,
  buildGoModule,
  go_1_25,
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
  buildPackages = if stdenv.isLinux then pkgsStatic else { inherit stdenv; buildGoModule = buildGoModule.override { go = go_1_25; }; };
  buildStdenv = buildPackages.stdenv;
  buildGoModule' = if stdenv.isLinux 
    then (buildPackages.buildGoModule.override { go = go_1_25; })
    else (buildGoModule.override { go = go_1_25; });

  tags =
    [
      "ledger"
      "netgo"
      "osusergo"
      "pebbledb"
    ]
    ++ lib.optionals nativeByteOrder [ "nativebyteorder" ]
    ++ lib.optionals buildStdenv.isDarwin [ "static_wasm" ]
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
  buildGoModule' rec {
  inherit
    pname
    version
    tags
    ldflags
    ;
  stdenv = buildStdenv;
  
  src = lib.sourceByRegex ../../.. [
    "^(evmd|ante|api|client|crypto|encoding|ethereum|ibc|indexer|mempool|metrics|precompiles|proto|rpc|server|testutil|utils|version|wallets|x|eips|contracts|go.mod|go.sum|interfaces.go)($|/.*)"
    "^tests(/.*[.]go)?$"
  ];
  
  vendorHash = null;
  proxyVendor = true;
  
  setSourceRoot = ''
    export sourceRoot=$(find . -maxdepth 2 -name "evmd" -type d | head -1)
    echo "evmd at: $sourceRoot"
  '';
  subPackages = [ "cmd/evmd" ];
  preBuild = ''
    mkdir -p $TMPDIR/lib
    export CGO_ENABLED=1
    export CGO_LDFLAGS="-L$TMPDIR/lib $CGO_LDFLAGS"
    export GOPROXY=https://proxy.golang.org,direct
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