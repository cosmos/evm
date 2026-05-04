const { expect } = require('chai')
const hre = require('hardhat')

describe('Staking', function () {
  it('should stake ATOM to a validator', async function () {
    const valAddr = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql'
    const stakeAmount = hre.ethers.parseEther('0.001')

    const staking = await hre.ethers.getContractAt(
      'StakingI',
      '0x0000000000000000000000000000000000000800'
    )

    const [signer] = await hre.ethers.getSigners()
    const tx = await staking
      .connect(signer)
      .delegate(signer, valAddr, stakeAmount)
    await new Promise(r => setTimeout(r, 200));
    await tx.wait(1)

    // Query delegation
    const delegation = await staking.delegation(signer, valAddr)
    expect(delegation.balance.amount).to.equal(
      stakeAmount,
      'Stake amount does not match'
    )
  })
})
