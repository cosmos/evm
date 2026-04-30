import { rm } from "node:fs/promises";
import { resolve } from "node:path";

// Keep wagmi-generated TS sources separate from build outputs.
// This directory is safe to delete and is regenerated on every build.
const generatedDir = resolve(process.cwd(), ".generated");

// Safety guard: only allow deleting the intended directories.
if (generatedDir !== resolve(process.cwd(), ".generated")) {
  throw new Error(`Refusing to delete unexpected path: ${generatedDir}`);
}

// Primary staging dir (new design)
await rm(generatedDir, { recursive: true, force: true });