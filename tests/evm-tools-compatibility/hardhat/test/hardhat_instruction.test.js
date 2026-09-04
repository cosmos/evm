const { execSync } = require("child_process");
const fs = require("fs");
const { expect } = require("chai");

describe("Hardhat Commands Compatibility", function () {
  it("Should compile contracts", function () {
    execSync("pnpm exec hardhat compile");
    expect(fs.existsSync("./artifacts")).to.be.true;
  });

  it("Should clean artifacts", function () {
    execSync("pnpm exec hardhat clean");
    expect(fs.existsSync("./artifacts")).to.be.false;
  });
  
  it("Should flatten contracts", function () {
    fs.mkdirSync("cache", { recursive: true });
    execSync("pnpm exec hardhat flatten contracts/TokenExample.sol > cache/Flattened.sol");
    expect(fs.existsSync("cache/Flattened.sol")).to.be.true;
  });

  it("Should run deploy script successfully", function () {
    execSync("pnpm exec hardhat compile");
    // execSync children don't inherit the outer suite's --network flag;
    // without it this deploys to an in-process network, not the node under test
    const output = execSync("pnpm exec hardhat run --no-compile --network localhost scripts/deploy.js").toString();
    console.log(output);
    expect(output).to.include("Token deployed to:");
  });
});
