{
  lib,
  stdenv,
  buildGo125Module,
  rev ? "dirty",
}:
buildGo125Module rec {
  version = "v0.5.0";
  pname = "evmd";
  tags = [
    "ledger"
    "ledger_zemu"
    "netgo" # pure go dns resolver
    "osusergo" # pure go user/group lookup
    "pebbledb"
  ];
  ldflags = [
    "-X github.com/cosmos/cosmos-sdk/version.Name=evmd"
    "-X github.com/cosmos/cosmos-sdk/version.AppName=${pname}"
    "-X github.com/cosmos/cosmos-sdk/version.Version=${version}"
    "-X github.com/cosmos/cosmos-sdk/version.BuildTags=${lib.concatStringsSep "," tags}"
    "-X github.com/cosmos/cosmos-sdk/version.Commit=${rev}"
  ];

  src = lib.sourceByRegex ./. [
    "^(evmd|ante|api|client|crypto|encoding|ethereum|ibc|indexer|mempool|metrics|precompiles|proto|rpc|server|testutil|utils|version|wallets|x|eips|contracts|go.mod|go.sum|interfaces.go)($|/.*)"
    "^tests(/.*[.]go)?$"
  ];
  vendorHash = "sha256-+L4nKIKHV1bos9Trr50/kG69hR8iNL6MXLi9mun5iXQ=";
  proxyVendor = true;
  env = {
    CGO_ENABLED = "1";
  };

  sourceRoot = "source/evmd";
  subPackages = [ "cmd/evmd" ];

  doCheck = false;
  meta = with lib; {
    description = "An EVM compatible framework for blockchain development with the Cosmos SDK";
    homepage = "https://github.com/cosmos/evm";
    license = licenses.asl20;
    mainProgram = "evmd" + stdenv.hostPlatform.extensions.executable;
    platforms = platforms.all;
  };
}
