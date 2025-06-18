const { expect } = require('chai')
const hre = require('hardhat')

describe('ERC20Factory', function () {
  it('should calculate the correct address', async function () {
    const salt = '0x4f5b6f778b28c4d67a9c12345678901234567890123456789012345678901234'
    const tokenPairType = 0
    const erc20Factory = await hre.ethers.getContractAt('IERC20Factory', '0x0000000000000000000000000000000000000900')
    const expectedAddress = await erc20Factory.calculateAddress(tokenPairType, salt)
    expect(expectedAddress).to.equal('0x6a040655fE545126cD341506fCD4571dB3A444F9')
  })

  it('should create a new ERC20 token', async function () {
    const salt = '0x4f5b6f778b28c4d67a9c12345678901234567890123456789012345678901234'
    const name = 'Test'
    const symbol = 'TEST'
    const decimals = 18
    const tokenPairType = 0

    const [signer] = await hre.ethers.getSigners()
    // Calculate the expected token address before deployment
    const erc20Factory = await hre.ethers.getContractAt('IERC20Factory', '0x0000000000000000000000000000000000000900')
    
    const tokenAddress = await erc20Factory.calculateAddress(tokenPairType, salt)
    const tx = await erc20Factory.connect(signer).create(tokenPairType, salt, name, symbol, decimals)

    // Get the token address from the transaction receipt
    const receipt = await tx.wait()
    expect(receipt.status).to.equal(1) // Check transaction was successful

    // Get the token contract instance
    const erc20Token = await hre.ethers.getContractAt('contracts/cosmos/erc20/IERC20.sol:IERC20', tokenAddress)

    // Verify token details through IERC20 queries
    expect(await erc20Token.totalSupply()).to.equal(0)
  })
})