/// <reference types="node" />
/**
 * Wagmi CLI: generates one .ts file per precompile from Hardhat artifacts.
 * Scans source (solidity/precompiles) for targets so config is never empty even before compile.
 * Run: pnpm run build:precompiles
 */
import { readdirSync, statSync, existsSync } from "fs";
import { join } from "path";
import { defineConfig } from "@wagmi/cli";
import { hardhat } from "@wagmi/cli/plugins";

const root = process.cwd();
const PRECOMPILES_SRC = join(root, "solidity", "precompiles");
const EXCLUDED_DIRS = ["testdata", "testutil"];
const EXCLUDED_CONTRACTS = ["Types"];

function* walkPrecompileSources(
  dir: string,
  prefix = ""
): Generator<{ module: string; contract: string }> {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (EXCLUDED_DIRS.includes(name)) continue;
      yield* walkPrecompileSources(full, join(prefix, name));
    } else if (name.endsWith(".sol")) {
      const contract = name.replace(".sol", "");
      if (EXCLUDED_CONTRACTS.includes(contract)) continue;
      yield { module: prefix, contract };
    }
  }
}

const precompiles = [...walkPrecompileSources(PRECOMPILES_SRC)];

const hardhatBase = {
  project: ".",
  artifacts: "artifacts",
  exclude: ["**/testdata/**", "**/testutil/**", "**/build-info/**", "**/*.dbg.json"],
};

export default defineConfig(
  precompiles.map(({ module, contract }) => ({
    out: join(".generated", "abi", "precompiles", module, `${contract}.ts`).replace(/\\/g, "/"),
    plugins: [
      hardhat({
        ...hardhatBase,
        include: [join("solidity", "precompiles", module, `${contract}.sol`, "*.json").replace(/\\/g, "/")],
      }),
    ],
  }))
);
