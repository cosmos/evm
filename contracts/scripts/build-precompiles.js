#!/usr/bin/env node
/**
 * Build precompile contracts (excluding testdata, testutil) in OpenZeppelin style:
 * - dist/precompiles/ : .sol sources
 * - dist/abi/         : ABI-only JSON per contract
 *
 * Run from contracts directory: pnpm run build:precompiles
 */

import { readFileSync, writeFileSync, mkdirSync, cpSync, existsSync, readdirSync, statSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";
import { execSync } from "child_process";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const ROOT = join(__dirname, "..");
const ARTIFACTS = join(ROOT, "artifacts", "solidity", "precompiles");
const SOLIDITY_SOURCE = join(ROOT, "solidity");
const DIST = join(ROOT, "dist");
const DIST_ABI = join(DIST, "abi");

const EXCLUDED_DIRS = ["testdata", "testutil"];

function ensureDir(p) {
  if (!existsSync(p)) mkdirSync(p, { recursive: true });
}

function* walkArtifactJsons(dir) {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (name.endsWith(".sol")) {
        const contractName = name.replace(".sol", "");
        const jsonPath = join(full, contractName + ".json");
        if (existsSync(jsonPath)) yield jsonPath;
      }
      yield* walkArtifactJsons(full);
    }
  }
}

function* walkSolSources(dir, prefix = "") {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      if (EXCLUDED_DIRS.includes(name)) continue;
      yield* walkSolSources(full, join(prefix, name));
    } else if (name.endsWith(".sol")) {
      yield join(prefix, name);
    }
  }
}

function buildPrecompiles() {
  if (!existsSync(ARTIFACTS)) {
    console.log("Compiling with Hardhat...");
    execSync("pnpm exec hardhat compile", {
      cwd: ROOT,
      stdio: "inherit",
    });
  }
  if (!existsSync(ARTIFACTS)) {
    console.error("No artifacts at", ARTIFACTS, "- run: pnpm exec hardhat compile");
    process.exit(1);
  }

  ensureDir(DIST_ABI);

  const copiedSol = new Set();
  let count = 0;

  for (const jsonPath of walkArtifactJsons(ARTIFACTS)) {
    const rel = jsonPath.slice(ARTIFACTS.length + 1);
    if (EXCLUDED_DIRS.some((d) => rel.includes(d))) continue;

    const artifact = JSON.parse(readFileSync(jsonPath, "utf8"));
    const sourceName = artifact.sourceName; // e.g. "solidity/precompiles/bank/IBank.sol"
    if (!sourceName || !sourceName.startsWith("solidity/precompiles/")) continue;

    const relFromSolidity = sourceName.replace(/^solidity\//, ""); // precompiles/bank/IBank.sol
    const solPath = join(SOLIDITY_SOURCE, relFromSolidity);
    const abiOutPath = join(DIST_ABI, relFromSolidity.replace(".sol", ".json"));
    const solOutPath = join(DIST, relFromSolidity);

    ensureDir(dirname(abiOutPath));
    ensureDir(dirname(solOutPath));

    writeFileSync(abiOutPath, JSON.stringify(artifact.abi ?? [], null, 2), "utf8");
    if (existsSync(solPath)) {
      cpSync(solPath, solOutPath);
      copiedSol.add(relFromSolidity);
    }
    count++;
    console.log("  ", relFromSolidity);
  }

  // Copy .sol sources that have no artifact (e.g. common/Types.sol with only structs)
  const precompilesSource = join(SOLIDITY_SOURCE, "precompiles");
  for (const rel of walkSolSources(precompilesSource)) {
    const relFromSolidity = join("precompiles", rel);
    if (copiedSol.has(relFromSolidity)) continue;
    const solPath = join(SOLIDITY_SOURCE, relFromSolidity);
    const solOutPath = join(DIST, relFromSolidity);
    ensureDir(dirname(solOutPath));
    cpSync(solPath, solOutPath);
    count++;
    console.log("  ", relFromSolidity, "(no ABI)");
  }

  console.log("\nDone. Built", count, "precompile file(s) to dist/ (OpenZeppelin style).");
}

buildPrecompiles();
