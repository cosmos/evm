// test/Web3jsCompatibility.test.js
const Web3 = require("web3");
const { expect } = require("chai");
const tokenArtifact = require("./contractABI/TokenExample.json");

describe("Web3.js Compatibility Test", function () {
  let web3, accounts, chainId, deployedToken;

  before(async function () {
    this.timeout(30000);
    web3 = new Web3("http://127.0.0.1:8545");
    accounts = await web3.eth.getAccounts();
    chainId = await web3.eth.getChainId();

    const Token = new web3.eth.Contract(tokenArtifact.abi);
    const deployTx = Token.deploy({ data: tokenArtifact.bytecode });
    deployedToken = await deployTx.send({ from: accounts[0], gas: 5000000 });
  });

  it("Should get chain ID", async function () {
    expect(chainId).to.equal(262144); // Adjust chainId as needed
  });

  it("Should fetch accounts", async function () {
    expect(accounts).to.be.an("array").that.is.not.empty;
  });

  it("Should read contract name", async function () {
    const name = await deployedToken.methods.name().call();
    expect(name).to.equal("Example");
  });

  it("Should mint tokens (send transaction)", async function () {
    this.timeout(30000);
    const receipt = await deployedToken.methods
      .mint(accounts[0], 1000)
      .send({ from: accounts[0], block: "latest", gas: 5000000 });
    const txHash = receipt.transactionHash;
    const confirmedReceipt = await web3.eth.getTransactionReceipt(txHash);
    expect(confirmedReceipt.status).to.be.true;
  });

  it("Should read balance (call)", async function () {
    const balance = await deployedToken.methods.balanceOf(accounts[0]).call();
    expect(Number(balance)).to.equal(1000);
  });

  it("Should query Transfer event logs", async function () {
    this.timeout(30000);
    
    // First perform a transfer to generate Transfer events
    const transferReceipt = await deployedToken.methods
      .transfer(accounts[1], 10)
      .send({ from: accounts[0], gas: 5000000 });
    
    // Query Transfer events using the block number from the transfer
    const events = await deployedToken.getPastEvents("Transfer", {
      fromBlock: transferReceipt.blockNumber - 10,
      toBlock: transferReceipt.blockNumber,
    });
    expect(events.length).to.be.greaterThan(0);
  });

  it("Should fail to transfer more tokens than balance (revert expected)", async function () {
    this.timeout(30000);

    const senderBalance = await deployedToken.methods
      .balanceOf(accounts[0])
      .call();
    const excessiveAmount = BigInt(senderBalance) + 1000n; // Try sending more than the current balance

    try {
      await deployedToken.methods
        .transfer(accounts[1], excessiveAmount.toString())
        .send({
          from: accounts[0],
          gas: 5000000,
        });
      // If no error is thrown, the test should fail
      expect.fail("Expected transfer to revert, but it succeeded");
    } catch (error) {
      // Revert expected, so we catch it here
      expect(error.message).to.satisfy(
        (msg) =>
          msg.includes("revert") ||
          msg.includes("VM Exception") ||
          msg.includes("exceeds balance")
      );
    }
  });
});
