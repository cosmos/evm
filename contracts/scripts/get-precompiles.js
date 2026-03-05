/**
 * Shared: list precompile contracts from Hardhat artifacts dir.
 * Used by build-precompiles.js and wagmi.config.ts (via import).
 */
import { readdirSync, statSync, existsSync } from "fs";
import { join } from "path";

const EXCLUDED_DIRS = ["testdata", "testutil"];

function* walk(dir, prefix = "") {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (EXCLUDED_DIRS.includes(name)) continue;
      if (name.endsWith(".sol")) {
        const contract = name.replace(".sol", "");
        if (existsSync(join(full, contract + ".json"))) yield { module: prefix, contract };
      } else {
        yield* walk(full, join(prefix, name));
      }
    }
  }
}

/** @param {string} artifactsPrecompilesDir path to artifacts/solidity/precompiles */
export function getPrecompiles(artifactsPrecompilesDir) {
  return [...walk(artifactsPrecompilesDir)];
}
