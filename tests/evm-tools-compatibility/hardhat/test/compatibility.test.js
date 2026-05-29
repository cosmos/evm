const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("Hardhat Local Chain Compatibility Test", function () {
  let Token;
  let token;
  let owner;
  let addr1;
  let addr2;

  this.beforeEach(async function () {
    [owner, addr1, addr2] = await ethers.getSigners();

    console.log("▶️ Accounts:");
    console.log("owner :", owner.address);
    console.log("addr1 :", addr1.address);
    console.log("addr2 :", addr2.address);

    const TokenFactory = await ethers.getContractFactory("TokenExample");
    token = await TokenFactory.deploy();
    await token.waitForDeployment();
    Token = token;

    const currentOwner = await Token.owner();
    console.log("✅ Contract deployed. Current owner:", currentOwner);
    expect(currentOwner).to.equal(owner.address);
  });

  it("Should deploy the contract correctly", async function () {
    expect(await Token.name()).to.equal("Example");
    expect(await Token.symbol()).to.equal("EXP");
  });

  it("Should mint initial supply to deployer", async function () {
    console.log("▶️ Minting 1,000,000 tokens to owner...");
    const tx = await Token.connect(owner).mint(owner.address, ethers.parseEther("1000000"));
    const receipt = await tx.wait();
    console.log("🔧 mint tx status:", receipt.status);
    expect(receipt.status).to.equal(1);

    const totalSupply = await Token.totalSupply();
    const balance = await Token.balanceOf(owner.address);
    console.log("🔎 totalSupply:", totalSupply.toString());
    console.log("🔎 owner balance:", balance.toString());

    expect(balance).to.equal(totalSupply);
  });

  it("Should allow transfers between accounts", async function () {
    console.log("▶️ MINT 1 token to owner");
    const mintTx = await Token.connect(owner).mint(owner.address, 1);
    await mintTx.wait();

    console.log("▶️ TRANSFER 1 token from owner to addr1");
    const tx = await Token.connect(owner).transfer(addr1.address, 1);
    const receipt = await tx.wait();
    console.log("🔧 transfer tx status:", receipt.status);
    expect(receipt.status).to.equal(1);

    const balanceOwner = await Token.balanceOf(owner.address);
    const balance1 = await Token.balanceOf(addr1.address);
    console.log("🔎 owner balance:", balanceOwner.toString());
    console.log("🔎 addr1 balance:", balance1.toString());

    expect(balance1).to.equal(1);
  });

  it("Should fail if sender doesn’t have enough balance", async function () {
    console.log("▶️ Attempting transfer from addr1 with no tokens...");
    // TODO: Update it to revertedWithCustomError when available (https://github.com/cosmos/evm/pull/289).
    await expect(
      Token.connect(addr1).transfer(addr2.address, 99999)
    ).to.be.reverted;

    console.log("✅ Transfer failed as expected due to insufficient balance.");
  });

  it("Should update balances after transfers", async function () {
    const amount = ethers.parseUnits("100000000000", 0);
    console.log(`▶️ Minting ${amount} tokens to owner...`);
    const mintTx = await Token.connect(owner).mint(owner.address, amount);
    await mintTx.wait();

    console.log("▶️ Transferring 500 tokens from owner to addr2...");
    const tx = await Token.connect(owner).transfer(addr2.address, 500);
    const receipt = await tx.wait();
    console.log("🔧 transfer tx status:", receipt.status);
    expect(receipt.status).to.equal(1);

    const balanceOwner = await Token.balanceOf(owner.address);
    const balance2 = await Token.balanceOf(addr2.address);
    const totalSupply = await Token.totalSupply();

    console.log("🔎 owner balance:", balanceOwner.toString());
    console.log("🔎 addr2 balance:", balance2.toString());
    console.log("🔎 totalSupply:", totalSupply.toString());

    expect(balance2).to.equal(500);
  });
});

