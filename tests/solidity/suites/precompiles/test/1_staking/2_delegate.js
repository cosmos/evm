const {expect} = require('chai')
const hre = require('hardhat')
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common')

describe('Staking – delegate with event assertion', function () {
    const STAKING_ADDRESS = '0x0000000000000000000000000000000000000800'
    const BECH32_ADDRESS = '0x0000000000000000000000000000000000000400'
    const GAS_LIMIT = 1_000_000 // skip gas estimation for simplicity

    let staking, bech32, signer

    before(async () => {
        [signer] = await hre.ethers.getSigners()

        // Instantiate the StakingI precompile
        staking = await hre.ethers.getContractAt('StakingI', STAKING_ADDRESS)
        // Instantiate the Bech32I precompile for address conversion
        bech32 = await hre.ethers.getContractAt('Bech32I', BECH32_ADDRESS)
    })

    it('should stake native coin and emit Delegate event (using precision-adjusted shares)', async function () {
        const valBech32 = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql'
        const stakeAmountBn = hre.ethers.parseEther('0.001')   // BigNumber
        const stakeAmount = BigInt(stakeAmountBn.toString())

        // compute the expected shares minted = stakeAmount * 10^18
        const precision = 10n ** 18n
        const stakeShares = stakeAmount * precision

        const hexValAddr = '0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E'

        // Query delegation before staking
        const beforeDelegation = await staking.delegation(signer.address, valBech32)
        const initialBalance = BigInt(beforeDelegation.balance.amount.toString())
        const initialShares = BigInt(beforeDelegation.shares.toString())
        console.log('Initial delegation balance:', initialBalance.toString())
        console.log('Initial delegation shares:', initialShares.toString())


        // Send the delegate tx
        const tx = await staking
            .connect(signer)
            .delegate(signer.address, valBech32, stakeAmount, {gasLimit: GAS_LIMIT})
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)
        console.log('Delegate tx hash:', receipt.hash, 'gas used:', receipt.gasUsed.toString())

        // parse the Delegate event from logs
        const delegateEvt = findEvent(receipt.logs, staking.interface, 'Delegate')
        expect(delegateEvt, 'Delegate event should be emitted').to.exist

        // verify event args
        expect(delegateEvt.args.delegatorAddress).to.equal(signer.address)
        expect(delegateEvt.args.validatorAddress).to.equal(hexValAddr)
        expect(BigInt(delegateEvt.args.amount.toString())).to.equal(stakeAmount)

        // ensure newShares ≥ initialShares + stakeShares
        const newShares = BigInt(delegateEvt.args.newShares.toString())
        expect(newShares).to.be.equal(stakeShares)

        // Query delegation after staking
        const afterDelegation = await staking.delegation(signer.address, valBech32)
        const afterBalance = BigInt(afterDelegation.balance.amount.toString())
        console.log('Delegated amount after staking:', afterBalance.toString())

        // ensure on-chain balance increased by exactly stakeAmount
        expect(afterBalance).to.equal(
            initialBalance + stakeAmount,
            'Delegation balance should increase by exactly stakeAmount'
        )
    })
})
