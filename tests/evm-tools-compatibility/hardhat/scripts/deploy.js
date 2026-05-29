const { ethers } = require("hardhat");

let deployedAddr;

async function deploy(){
    const [deployer] = await ethers.getSigners();
    console.log("Deployer:", deployer.address);
  
    const Contract = await ethers.getContractFactory("TokenExample");
    const exampleToken=await Contract.deploy();

    await exampleToken.waitForDeployment();
    const addr = await exampleToken.getAddress();
    console.log('Token deployed to:'+addr)
    deployedAddr = addr;


}

async function main(){
    await deploy();
}

main();