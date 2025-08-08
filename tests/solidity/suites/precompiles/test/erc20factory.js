const { expect } = require('chai')
const hre = require('hardhat')

const abi = [
  "function create(uint8 tokenPairType, bytes32 salt, string memory name, string memory symbol, uint8 decimals, address minter, uint256 premintedSupply) external returns (address)",
  "function calculateAddress(uint8 tokenPairType, bytes32 salt) external view returns (address)",
  "event Create(address indexed tokenAddress, uint8 tokenPairType, bytes32 salt, string name, string symbol, uint8 decimals, address minter, uint256 premintedSupply)"
]

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
    const premintedSupply = hre.ethers.parseEther("1000000") // 1M tokens

    const [signer] = await hre.ethers.getSigners()
    const minter = signer.address

    // Calculate the expected token address before deployment
    const erc20Factory = await hre.ethers.getContractAt('IERC20Factory', '0x0000000000000000000000000000000000000900')

    const tokenAddress = await erc20Factory.calculateAddress(tokenPairType, salt)
    const tx = await erc20Factory.connect(signer).create(tokenPairType, salt, name, symbol, decimals, minter, premintedSupply)

    // Get the token address from the transaction receipt
    const receipt = await tx.wait()
    expect(receipt.status).to.equal(1) // Check transaction was successful

    // Create a contract instance with the full ABI including events for event filtering
    const erc20FactoryWithEvents = new hre.ethers.Contract('0x0000000000000000000000000000000000000900', abi, signer)

    // Get the Create event from the transaction receipt
    const createEvents = await erc20FactoryWithEvents.queryFilter(erc20FactoryWithEvents.filters.Create(), receipt.blockNumber, receipt.blockNumber)
    expect(createEvents.length).to.equal(1)
    expect(createEvents[0].args.tokenAddress).to.equal(tokenAddress)
    expect(createEvents[0].args.tokenPairType).to.equal(tokenPairType)
    expect(createEvents[0].args.salt).to.equal(salt)
    expect(createEvents[0].args.name).to.equal(name)
    expect(createEvents[0].args.symbol).to.equal(symbol)
    expect(createEvents[0].args.decimals).to.equal(decimals)
    expect(createEvents[0].args.minter).to.equal(minter)
    expect(createEvents[0].args.premintedSupply).to.equal(premintedSupply)

    // Get the token contract instance
    const erc20Token = await hre.ethers.getContractAt('contracts/cosmos/erc20/IERC20.sol:IERC20', tokenAddress)

    // Verify token details through IERC20 queries
    expect(await erc20Token.totalSupply()).to.equal(premintedSupply)
  })
})