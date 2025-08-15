// test/EthersFeatures.test.js
const { ethers } = require("hardhat");
const { expect } = require("chai");

describe("Ethers.js Core Features Compatibility Test", function () {
  let token;
  let owner, addr1;
  let tx;

  beforeEach(async function () {
    [owner, addr1] = await ethers.getSigners();

    const Token = await ethers.getContractFactory("TokenExample");
    token = await Token.deploy();
    await token.waitForDeployment();
  });

  it("Should fetch signer and balance", async function () {
    const balance = await ethers.provider.getBalance(owner.address);
    expect(balance).to.be.a("bigint");
  });

  it("Should send native ETH between signers", async function () {
    const tx = await owner.sendTransaction({
      to: addr1.address,
      value: ethers.parseEther("1.0")
    });
    const receipt = await tx.wait();
    expect(receipt.status).to.equal(1);
  });

  it("Should allow owner to mint tokens", async function () {
    const tx = await token.mint(addr1.address, 1000);
    const receipt = await tx.wait();
    expect(receipt.status).to.equal(1);

    const balance = await token.balanceOf(addr1.address);
    expect(balance).to.equal(1000);
  });

  it("Should revert mint if called by non-owner", async function () {
    // TODO: Update it to revertedWithCustomError when available (https://github.com/cosmos/evm/pull/289).
    await expect(token.connect(addr1).mint(addr1.address, 1000)).to.be.reverted;
  });

  it("Should estimate gas for a mint transaction", async function () {
    const gas = await token.mint.estimateGas(addr1.address, 500);
    expect(gas).to.be.a("bigint");
  });

  it("Should encode and decode ABI manually for mint", async function () {
    const iface = new ethers.Interface(["function mint(address to, uint256 amount)"]);
    const data = iface.encodeFunctionData("mint", [addr1.address, 123]);
    const decoded = iface.decodeFunctionData("mint", data);

    expect(decoded[0]).to.equal(addr1.address);
    expect(decoded[1]).to.equal(123n);
  });

  it("Should get transaction and wait for confirmation (mint)", async function () {
    tx = await token.mint(addr1.address, 10);
    const receipt = await tx.wait();
    expect(receipt.status).to.equal(1);
  });

  it("Should listen and respond to Transfer event from mint", async function () {
    return new Promise(async (resolve, reject) => {
      token.once("Transfer", (from, to, value) => {
        try {
          expect(from).to.equal(ethers.ZeroAddress);
          expect(to).to.equal(addr1.address);
          expect(value).to.equal(77n);
          resolve();
        } catch (e) {
          reject(e);
        }
      });
      const tx = await token.mint(addr1.address, 77);
      const receipt = await tx.wait();
      expect(receipt.status).to.equal(1);
    });
  });

  it("Should decode mint-related Transfer event log using interface", async function () {
    const tx = await token.mint(addr1.address, 42);
    const receipt = await tx.wait();
    expect(receipt.status).to.equal(1);

    const iface = new ethers.Interface(["event Transfer(address indexed from, address indexed to, uint256 value)"]);
    const mintLog = receipt.logs.find((log) => log.address === token.target);
    const parsed = iface.parseLog(mintLog);

    expect(parsed.name).to.equal("Transfer");
    expect(parsed.args.from).to.equal(ethers.ZeroAddress);
    expect(parsed.args.to).to.equal(addr1.address);
    expect(parsed.args.value).to.equal(42n);
  });

  it("Should query past Transfer events using queryFilter", async function () {
    const tx = await token.mint(addr1.address, 88);
    const receipt = await tx.wait();
    expect(receipt.status).to.equal(1);

    const filter = token.filters.Transfer(ethers.ZeroAddress, addr1.address);
    const logs = await token.queryFilter(filter, receipt.blockNumber, receipt.blockNumber);

    expect(logs.length).to.be.greaterThan(0);
    const latest = logs[logs.length - 1];
    expect(latest.args.to).to.equal(addr1.address);
    expect(latest.args.from).to.equal(ethers.ZeroAddress);
    expect(latest.args.value).to.equal(88n);
  });
});
