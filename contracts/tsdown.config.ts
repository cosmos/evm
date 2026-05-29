import { defineConfig } from "tsdown";

export default defineConfig({
  entry: ["./.generated/precompiles/**/*.ts", "!./.generated/precompiles/**/*.d.ts"],
  format: ["esm", "cjs"],
  outDir: "dist/precompiles",
  dts: true,
  unbundle: true,
  clean: true,
  outExtensions: ({ format }) => ({
    js: format === 'cjs' ? '.cjs' : '.js',
    dts: '.d.ts',
  }),
  platform: 'neutral',
  copy: [
    {
      from: [
        "solidity/precompiles/**/*.sol",
        "!solidity/precompiles/**/testdata/**",
        "!solidity/precompiles/**/testutil/**",
      ],
      to: ".",
      flatten: false,
      verbose: true,
    },
  ],
});
