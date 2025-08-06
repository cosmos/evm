const {expect} = require('chai')
const hre = require('hardhat')
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common')

function formatUnbondingDelegation(res) {
    const delegatorAddress = res[0]
    const validatorAddress = res[1]
    const rawEntries = res[2] // array of Result(6)

    const entries = rawEntries.map(entry => {
        const [
            creationHeight,
            completionTime,
            initialBalance,
            balance,
            unbondingId,
            unbondingOnHoldRefCount,
        ] = entry

        return {
            creationHeight: Number(creationHeight),
            completionTime: Number(completionTime),
            initialBalance: BigInt(initialBalance.toString()),
            balance: BigInt(balance.toString()),
            unbondingId: Number(unbondingId),
            unbondingOnHoldRefCount: Number(unbondingOnHoldRefCount),
        }
    })

    return {
        delegatorAddress,
        validatorAddress,
        entries,
    }
}

describe('Staking – delegate, undelegate & cancelUnbondingDelegation with event assertions', function () {
    const STAKING_ADDRESS = '0x0000000000000000000000000000000000000800'
    const GAS_LIMIT = 1_000_000 // skip gas estimation for simplicity

    let staking, signer

    before(async () => {
        [signer] = await hre.ethers.getSigners()
        // Instantiate the StakingI precompile contract
        staking = await hre.ethers.getContractAt('StakingI', STAKING_ADDRESS)
    })

    it('should delegate, undelegate, then cancel unbonding and emit correct events', async function () {
        const valBech32 = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql'
        const amount = hre.ethers.parseEther('0.001')

        // DELEGATE
        const delegateTx = await staking.connect(signer).delegate(signer.address, valBech32, amount, {gasLimit: GAS_LIMIT})
        const delegateReceipt = await waitWithTimeout(delegateTx, 20000, RETRY_DELAY_FUNC)
        console.log('Delegate tx hash:', delegateTx.hash, 'gas used:', delegateReceipt.gasUsed.toString())

        const hexValAddr = '0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E'
        const delegateEvt = findEvent(delegateReceipt.logs, staking.interface, 'Delegate')
        expect(delegateEvt, 'Delegate event should be emitted').to.exist
        expect(delegateEvt.args.delegatorAddress).to.equal(signer.address)
        expect(delegateEvt.args.validatorAddress).to.equal(hexValAddr)
        expect(delegateEvt.args.amount).to.equal(amount)

        // COUNT UNBONDING ENTRIES BEFORE
        const beforeRaw = await staking.unbondingDelegation(signer.address, valBech32)
        const entriesBefore = formatUnbondingDelegation(beforeRaw).entries.length

        // UNDELEGATE
        const undelegateTx = await staking.connect(signer).undelegate(signer.address, valBech32, amount, {gasLimit: GAS_LIMIT})
        const undelegateReceipt = await waitWithTimeout(undelegateTx, 20000, RETRY_DELAY_FUNC)
        console.log('Undelegate tx hash:', undelegateTx.hash, 'gas used:', undelegateReceipt.gasUsed.toString())

        const unbondEvt = findEvent(undelegateReceipt.logs, staking.interface, 'Unbond')
        expect(unbondEvt, 'Unbond event should be emitted').to.exist
        expect(unbondEvt.args.delegatorAddress).to.equal(signer.address)
        expect(unbondEvt.args.validatorAddress).to.equal(hexValAddr)
        expect(unbondEvt.args.amount).to.equal(amount)
        const completionTime = BigInt(unbondEvt.args.completionTime.toString())
        expect(completionTime > 0n, 'completionTime should be positive').to.be.true

        // COUNT UNBONDING ENTRIES AFTER
        const afterRaw = await staking.unbondingDelegation(signer.address, valBech32)
        const afterUnbonding = formatUnbondingDelegation(afterRaw)
        console.log('Unbonding Delegation:', afterUnbonding)
        const entriesAfter = afterUnbonding.entries.length

        expect(entriesAfter).to.equal(
            entriesBefore + 1,
            'Number of unbonding entries should increase by 1'
        )
        expect(afterUnbonding.entries[0].balance).to.equal(
            BigInt(amount.toString()),
            'Unbonding entry balance should match undelegated amount'
        )

        // CANCEL UNBONDING DELEGATION
        const entryToCancel = afterUnbonding.entries[0]
        const cancelTx = await staking.connect(signer).cancelUnbondingDelegation(
            signer.address,
            valBech32,
            amount,
            entryToCancel.creationHeight,
            {gasLimit: GAS_LIMIT}
        )
        const cancelReceipt = await waitWithTimeout(cancelTx, 20000, RETRY_DELAY_FUNC)
        console.log('CancelUnbondingDelegation tx hash:', cancelTx.hash, 'gas used:', cancelReceipt.gasUsed.toString())

        const cancelEvt = findEvent(cancelReceipt.logs, staking.interface, 'CancelUnbondingDelegation')
        expect(cancelEvt, 'CancelUnbondingDelegation event should be emitted').to.exist
        expect(cancelEvt.args.delegatorAddress).to.equal(signer.address)
        expect(cancelEvt.args.validatorAddress).to.equal(hexValAddr)
        expect(cancelEvt.args.amount).to.equal(amount)
        expect(cancelEvt.args.creationHeight).to.equal(entryToCancel.creationHeight)

        // VERIFY ENTRY REMOVAL
        const finalRaw = await staking.unbondingDelegation(signer.address, valBech32)
        const finalEntries = formatUnbondingDelegation(finalRaw).entries.length
        console.log('Unbonding Delegation after cancel:', finalRaw)
        expect(finalEntries).to.equal(
            entriesBefore,
            'Number of unbonding entries should return to original count after cancellation'
        )
    })
})
