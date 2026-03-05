#!/usr/bin/env node
/**
 * Build precompile contracts (excluding testdata, testutil):
 * - dist/precompiles/ : .sol sources only (this script)
 * - dist/abi/         : .ts from wagmi generate
 *
 * Run from contracts: pnpm run build
 */

import { mkdirSync, cpSync, existsSync, readdirSync, statSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
import { execSync } from "child_process";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const ROOT = join(__dirname, "..");
const ARTIFACTS = join(ROOT, "artifacts", "solidity", "precompiles");
const SOLIDITY_SOURCE = join(ROOT, "solidity", "precompiles");
const DIST = join(ROOT, "dist");

const EXCLUDED_DIRS = ["testdata", "testutil"];

function ensureDir(p) {
  if (!existsSync(p)) mkdirSync(p, { recursive: true });
}

function* walkSolSources(dir, prefix = "") {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (EXCLUDED_DIRS.includes(name)) continue;
      yield* walkSolSources(full, join(prefix, name));
    } else if (name.endsWith(".sol")) {
      yield { rel: join(prefix, name), solPath: full };
    }
  }
}

function buildPrecompiles() {
  // So that wagmi generate can run, ensure artifacts exist
  if (!existsSync(ARTIFACTS)) {
    console.log("Compiling with Hardhat...");
    execSync("pnpm exec hardhat compile", {
      cwd: ROOT,
      stdio: "inherit",
    });
  }

  let count = 0;
  for (const { rel, solPath } of walkSolSources(SOLIDITY_SOURCE)) {
    const relFromPrecompiles = join("precompiles", rel);
    const solOutPath = join(DIST, relFromPrecompiles);
    ensureDir(dirname(solOutPath));
    cpSync(solPath, solOutPath);
    count++;
    console.log("  ", relFromPrecompiles);
  }

  console.log("\nDone. Copied", count, ".sol file(s) to dist/precompiles/.");
}

buildPrecompiles();
