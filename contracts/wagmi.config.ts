/// <reference types="node" />
/**
 * Wagmi CLI config: generate one .ts file per precompile from Hardhat artifacts.
 * Precompile list from scripts/get-precompiles.js (same logic as build-precompiles).
 * Run: pnpm run build:precompiles (script then wagmi generate).
 */
import { join } from "path";
import { getPrecompiles } from "./scripts/get-precompiles.js";
import { defineConfig } from "@wagmi/cli";
import { hardhat } from "@wagmi/cli/plugins";

// Wagmi runs with cwd = config dir (contracts/)
const root = process.cwd();
const PRECOMPILES = getPrecompiles(join(root, "artifacts", "solidity", "precompiles"));

const hardhatBase = {
  project: ".",
  artifacts: "artifacts",
  exclude: ["**/testdata/**", "**/testutil/**", "**/build-info/**", "**/*.dbg.json"],
};

export default defineConfig(
  PRECOMPILES.map(({ module, contract }) => ({
    out: `dist/abi/precompiles/${module}/${contract}.ts`,
    plugins: [
      hardhat({
        ...hardhatBase,
        include: [`solidity/precompiles/${module}/${contract}.sol/*.json`],
      }),
    ],
  }))
);
