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
    execSync("pnpm exec hardhat flatten contracts/TokenExample.sol > Flattened.sol");
    expect(fs.existsSync("Flattened.sol")).to.be.true;
  });
  
  it("Should run deploy script successfully", function () {
    execSync("pnpm exec hardhat compile");
    const output = execSync("pnpm exec hardhat run --no-compile scripts/deploy.js").toString();
    console.log(output);
    expect(output).to.include("Token deployed to:");
  });

  it("Should run deploy via hardhat-deploy", function () {
    const output = execSync("pnpm exec hardhat deploy").toString();
    console.log(output);
    expect(output).to.include("deploying \"TokenExample\"");
  });
});

