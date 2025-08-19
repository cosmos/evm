// test/UniswapV3RouterManagerDeploy.test.js
const { ethers } = require("hardhat");
const { expect } = require("chai");
const factoryArtifact = require("./abis/v3-core/UniswapV3Factory.sol/UniswapV3Factory.json");
const routerArtifact = require("./abis/v3-periphery/SwapRouter.sol/SwapRouter.json");
const managerArtifact = require("./abis/v3-periphery/NonfungiblePositionManager.sol/NonfungiblePositionManager.json");
const descriptorArtifact = require("./abis/v3-periphery/NonfungibleTokenPositionDescriptor.sol/NonfungibleTokenPositionDescriptor.json");
const descriptorLibArtifact = require("./abis/v3-periphery/libraries/NFTDescriptor.sol/NFTDescriptor.json");

describe("Uniswap V3 Router and Manager Deployment Test", function () {
    let factory, router, manager, descriptor, weth;
    let token0, token1, wethAddr, factoryAddr, descriptorAddr;

    before(async function () {
        this.timeout(180000);
        const [deployer] = await ethers.getSigners();

        // Deploy Factory
        const Factory = await ethers.getContractFactory(factoryArtifact.abi, factoryArtifact.bytecode, deployer);
        factory = await Factory.deploy();
        await factory.waitForDeployment();
        factoryAddr = await factory.getAddress();
        console.log("factory: " + factoryAddr);

        // Deploy Tokens
        const Token = await ethers.getContractFactory("TestToken", deployer);
        token0 = await Token.deploy("MockUSDC", "mUSDC");
        await token0.waitForDeployment();
        token1 = await Token.deploy("MockUSDT", "mUSDT");
        await token1.waitForDeployment();
        console.log("token0: " + await token0.getAddress());
        console.log("token1: " + await token1.getAddress());
        // Deploy WETH9 Mock
        const WETH9 = await ethers.getContractFactory("WETH9Mock", deployer);
        weth = await WETH9.deploy();
        await weth.waitForDeployment();
        wethAddr = await weth.getAddress();
        console.log("WETH9: " + wethAddr);
        console.log("WETH9 Address:", wethAddr, typeof wethAddr);

        const NFTDescriptorFactory = await ethers.getContractFactory(descriptorLibArtifact.abi,descriptorLibArtifact.bytecode,deployer);
        const nftDescriptor = await NFTDescriptorFactory.deploy();
        await nftDescriptor.waitForDeployment();
        const nftDescriptorAddr = await nftDescriptor.getAddress();
        console.log("lib: "+nftDescriptorAddr);
        const Descriptor=await ethers.getContractFactoryFromArtifact(
            descriptorArtifact,
            {
                signer:deployer,
                libraries: {
                    NFTDescriptor: nftDescriptorAddr
                }
            }
        );
        const label = await ethers.encodeBytes32String("ETH");
        console.log("Descriptor Label:", label, typeof label);

        descriptor = await Descriptor.deploy(wethAddr, label);
        await descriptor.waitForDeployment();
        descriptorAddr = await descriptor.getAddress();
        console.log("descriptor: " + descriptorAddr);

        // Deploy Manager
        const Manager = await ethers.getContractFactory(managerArtifact.abi, managerArtifact.bytecode, deployer);
        manager = await Manager.deploy(factoryAddr, wethAddr, descriptorAddr);
        await manager.waitForDeployment();

        // Deploy Router
        const Router = await ethers.getContractFactory(routerArtifact.abi, routerArtifact.bytecode, deployer);
        router = await Router.deploy(factoryAddr, wethAddr);
        await router.waitForDeployment();
    });

    it("Should deploy pool from factory successfully", async function () {
        this.timeout(30000);
        const token0Addr=await token0.getAddress()
        const token1Addr=await token1.getAddress()
        const tx=await factory.createPool(token0Addr,token1Addr,3000);
        const receipt=await tx.wait();
        expect(receipt.status).to.equal(1);
    });

    it("Should deploy SwapRouter successfully", async function () {
        this.timeout(30000);
        const address = await router.getAddress();
        expect(address).to.properAddress;
    });

    it("Should deploy NonfungiblePositionManager successfully", async function () {
        this.timeout(30000);
        const address = await manager.getAddress();
        expect(address).to.properAddress;
    });

    it("Should deploy NonfungibleTokenPositionDescriptor successfully", async function () {
        this.timeout(30000);
        const address = await descriptor.getAddress();
        expect(address).to.properAddress;
    });
});
