import { rm } from "node:fs/promises";
import { resolve } from "node:path";

// Keep generated inputs and published Solidity interfaces in sync with sources.
// These directories are safe to delete and are regenerated on every build.
const generatedDir = resolve(process.cwd(), ".generated");
const precompilesDir = resolve(process.cwd(), "precompiles");

// Safety guard: only allow deleting the intended directories.
if (generatedDir !== resolve(process.cwd(), ".generated")) {
  throw new Error(`Refusing to delete unexpected path: ${generatedDir}`);
}
if (precompilesDir !== resolve(process.cwd(), "precompiles")) {
  throw new Error(`Refusing to delete unexpected path: ${precompilesDir}`);
}

await rm(generatedDir, { recursive: true, force: true });
await rm(precompilesDir, { recursive: true, force: true });

