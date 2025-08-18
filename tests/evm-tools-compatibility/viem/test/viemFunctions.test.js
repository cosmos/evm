// test/ViemFunctions.test.js
const {
  createPublicClient,
  createWalletClient,
  http,
} = require("viem");
const { privateKeyToAccount } = require("viem/accounts");
const { expect } = require("chai");
const tokenArtifact = require("./contractABI/TokenExample.json");

require("dotenv").config();
const chainId = process.env.CHAIN_ID || 262144; // Default to 262144 if not set

describe("Viem Full Feature Test", function () {
  let publicClient,
    walletClient,
    walletAccount,
    contractAddress,
    lastTxHash,
    accounts;

  before(async function () {
    this.timeout(30000);
    publicClient = createPublicClient({
      chain: { id: chainId },
      transport: http("http://127.0.0.1:8545"),
    });

    const privateKey = process.env.PRIVATE_KEY;
    walletAccount = privateKeyToAccount(
      privateKey.startsWith("0x") ? privateKey : "0x" + privateKey
    );

    walletClient = createWalletClient({
      account: walletAccount,
      chain: { id: chainId },
      transport: http("http://127.0.0.1:8545"),
    });

    const deploymentTxHash = await walletClient.deployContract({
      abi: tokenArtifact.abi,
      bytecode: tokenArtifact.bytecode,
      args: [],
    });
    const receipt = await publicClient.waitForTransactionReceipt({
      hash: deploymentTxHash,
    });
    contractAddress = receipt.contractAddress;

    const addresses = await publicClient.request({ method: "eth_accounts" });
    accounts = addresses.slice(0, 6); // 0번 ~ 5번 계정
    console.log("Accounts:", accounts);
  });

  it("Should get chain ID and block number", async function () {
    const actualChainId = await publicClient.getChainId();
    expect(actualChainId).to.equal(chainId);

    const blockNumber = await publicClient.getBlockNumber();
    expect(blockNumber).to.be.a("bigint");
  });

  it("Should estimate gas for mint", async function () {
    const gas = await publicClient.estimateGas({
      account: walletAccount.address,
      address: contractAddress,
      abi: tokenArtifact.abi,
      functionName: "mint",
      args: [walletAccount.address, 100],
    });
    expect(gas).to.be.a("bigint");
  });

  it("Should mint tokens and get transaction details", async function () {
    this.timeout(30000);
    const { request } = await publicClient.simulateContract({
      account: walletAccount,
      address: contractAddress,
      abi: tokenArtifact.abi,
      functionName: "mint",
      args: [walletAccount.address, 200],
    });

    lastTxHash = await walletClient.writeContract(request);

    await publicClient.waitForTransactionReceipt({ hash: lastTxHash });
    const tx = await publicClient.getTransaction({ hash: lastTxHash });
    expect(tx.hash).to.equal(lastTxHash);

    const receipt = await publicClient.getTransactionReceipt({
      hash: lastTxHash,
    });
    expect(receipt.status).to.equal("success");
  });

  it("Should check balance after mint", async function () {
    const balance = await publicClient.readContract({
      address: contractAddress,
      abi: tokenArtifact.abi,
      functionName: "balanceOf",
      args: [walletAccount.address],
    });
    expect(balance).to.equal(200n);
  });

  it("Should query Transfer event logs", async function () {
    this.timeout(30000);
    const { request: transferRequest } = await publicClient.simulateContract({
      account: walletAccount,
      address: contractAddress,
      abi: tokenArtifact.abi,
      functionName: "transfer",
      args: [accounts[0], 10n],
    });

    const txHash1 = await walletClient.writeContract(transferRequest);

    const Txreceipt1 = await publicClient.waitForTransactionReceipt({
      hash: txHash1,
    });
    
    // Get logs with block range to ensure we capture the recent transaction
    const logs = await publicClient.getLogs({
      abi: tokenArtifact.abi,
      address: contractAddress,
      eventName: "Transfer",
      fromBlock: Txreceipt1.blockNumber - 10n, // Look back a few blocks
      toBlock: Txreceipt1.blockNumber,
    });
    expect(logs.length).to.be.greaterThan(0);
  });

  it("Should encode and decode ABI parameters", function () {
    const { encodeAbiParameters, decodeAbiParameters } = require("viem");
    const encoded = encodeAbiParameters(
      [
        { name: "amount", type: "uint256" },
        { name: "recipient", type: "address" },
      ],
      [500, walletAccount.address]
    );

    const decoded = decodeAbiParameters(
      [
        { name: "amount", type: "uint256" },
        { name: "recipient", type: "address" },
      ],
      encoded
    );

    expect(decoded[0]).to.equal(500n);
    expect(decoded[1]).to.equal(walletAccount.address);
  });

  it("Should revert if transferring more tokens than balance", async function () {
    const invalidAmount = 9999999n;

    try {
      const { request: bigTransferRequest } =
        await publicClient.simulateContract({
          account: walletAccount,
          address: contractAddress,
          abi: tokenArtifact.abi,
          functionName: "transfer",
          args: [accounts[1], invalidAmount],
        });
      await walletClient.writeContract(bigTransferRequest);

      expect.fail("Expected 'transfer' to revert but it succeeded.");
    } catch (error) {
      if (error.cause?.data?.errorName) {
        // Check custom error name and arguments
        expect(error.cause.data.errorName).to.equal("ERC20InsufficientBalance");
        // For instance, check the 'needed' amount:
        // error.cause.data.args = [sender, balance, needed]
        expect(error.cause.data.args[2]).to.equal(invalidAmount);
      } else {
        // Fallback if not a custom error
        expect(error.message).to.include(
          'The contract function "transfer" reverted'
        );
      }
    }
  });
});
