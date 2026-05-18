const {expect} = require('chai')
const hre = require('hardhat')
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common')

// Cosmos SDK LegacyDec precision (18 decimal places)
const PRECISION = 10n ** 18n

/**
 * Convert the raw tuple from staking.delegation(...)
 * into an object that mirrors the DelegationOutput struct.
 * @param {*} res - Raw delegation response: [shares, Coin]
 */
function formatDelegation(res) {
    const shares = BigInt(res[0].toString())
    const balance = {
        denom: res[1][0],
        amount: BigInt(res[1][1].toString())
    }
    
    return {
        shares,
        balance
    }
}

/**
 * Convert the raw tuple from staking.redelegation(...)
 * into an object that mirrors the RedelegationOutput struct.
 */
function formatRedelegation(res) {
    const delegatorAddress = res[0]
    const validatorSrcAddress = res[1]
    const validatorDstAddress = res[2]
    const rawEntries = res[3] // array of RedelegationEntry

    const entries = rawEntries.map(entry => {
        const [
            creationHeight,
            completionTime,
            initialBalance,
            sharesDst,
        ] = entry

        return {
            creationHeight: Number(creationHeight),
            completionTime: Number(completionTime),
            initialBalance: BigInt(initialBalance.toString()),
            sharesDst: BigInt(sharesDst.toString()),
        }
    })

    return {
        delegatorAddress,
        validatorSrcAddress,
        validatorDstAddress,
        entries,
    }
}

describe('Staking – redelegate with event and state assertions', function () {
    const STAKING_ADDRESS = '0x0000000000000000000000000000000000000800'
    const GAS_LIMIT = 1_000_000 // skip gas estimation for simplicity

    let staking, signer

    before(async () => {
        [signer] = await hre.ethers.getSigners()
        // instantiate StakingI and Bech32I precompile contracts
        staking = await hre.ethers.getContractAt('StakingI', STAKING_ADDRESS)
    })

    it('should redelegate tokens and emit Redelegate event', async function () {
        const signerBech32 = 'cosmos1cml96vmptgw99syqrrz8az79xer2pcgp95srxm'
        const srcValBech32 = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql'
        const dstValBech32 = 'cosmosvaloper1cml96vmptgw99syqrrz8az79xer2pcgpqqyk2g'

        // decode bech32 → hex for event comparisons
        const srcValHex = '0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E'
        const dstValHex = '0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101'

        // 1) query current delegations to both validators before redelegation
        const beforeSrcDelegationRaw = await staking.delegation(signer.address, srcValBech32)
        const beforeDstDelegationRaw = await staking.delegation(signer.address, dstValBech32)
        const beforeSrcDelegation = formatDelegation(beforeSrcDelegationRaw)
        const beforeDstDelegation = formatDelegation(beforeDstDelegationRaw)
        const amount = beforeSrcDelegation.balance.amount
        
        console.log('Before redelegation - srcVal delegation shares:', beforeSrcDelegation.shares.toString())
        console.log('Before redelegation - srcVal delegation balance:', beforeSrcDelegation.balance.amount.toString(), beforeSrcDelegation.balance.denom)
        console.log('Before redelegation - dstVal delegation shares:', beforeDstDelegation.shares.toString())
        console.log('Before redelegation - dstVal delegation balance:', beforeDstDelegation.balance.amount.toString(), beforeDstDelegation.balance.denom)

        // 2) query redelegation entries before
        const beforeRaw = await staking.redelegation(signer.address, srcValBech32, dstValBech32)
        const beforeR = formatRedelegation(beforeRaw)
        const entriesBefore = beforeR.entries.length

        // 3) send the redelegate transaction
        const tx = await staking
            .connect(signer)
            .redelegate(signer.address, srcValBech32, dstValBech32, amount, {gasLimit: GAS_LIMIT})
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)
        console.log('Redelegate tx hash:', tx.hash, 'gas used:', receipt.gasUsed.toString())

        // 4) parse and assert the Redelegate event
        const redelegateEvt = findEvent(receipt.logs, staking.interface, 'Redelegate')
        expect(redelegateEvt, 'Redelegate event should be emitted').to.exist
        expect(redelegateEvt.args.delegatorAddress).to.equal(signer.address)
        expect(redelegateEvt.args.validatorSrcAddress).to.equal(srcValHex)
        expect(redelegateEvt.args.validatorDstAddress).to.equal(dstValHex)
        expect(redelegateEvt.args.amount).to.equal(amount)
        const completionTime = BigInt(redelegateEvt.args.completionTime.toString())
        expect(completionTime > 0n, 'completionTime should be positive').to.be.true

        // 5) query delegations after redelegation to verify state changes
        const afterSrcDelegationRaw = await staking.delegation(signer.address, srcValBech32)
        const afterDstDelegationRaw = await staking.delegation(signer.address, dstValBech32)
        const afterSrcDelegation = formatDelegation(afterSrcDelegationRaw)
        const afterDstDelegation = formatDelegation(afterDstDelegationRaw)
        
        console.log('After redelegation - srcVal delegation shares:', afterSrcDelegation.shares.toString())
        console.log('After redelegation - srcVal delegation balance:', afterSrcDelegation.balance.amount.toString(), afterSrcDelegation.balance.denom)
        console.log('After redelegation - dstVal delegation shares:', afterDstDelegation.shares.toString())
        console.log('After redelegation - dstVal delegation balance:', afterDstDelegation.balance.amount.toString(), afterDstDelegation.balance.denom)

        // Assert balance changes
        expect(afterSrcDelegation.balance.amount).to.equal(
            beforeSrcDelegation.balance.amount - amount,
            'Source validator delegation balance should decrease by redelegated amount'
        )
        expect(afterDstDelegation.balance.amount).to.equal(
            beforeDstDelegation.balance.amount + amount,
            'Destination validator delegation balance should increase by redelegated amount'
        )

        // Calculate expected shares changes (accounting for 18-decimal precision)
        // Shares = amount * 10^18 (LegacyDec precision)
        const amountWithPrecision = amount * PRECISION
        
        // When redelegating the full amount, source validator shares should become 0
        const expectedSrcShares = beforeSrcDelegation.balance.amount === amount ? 0n : beforeSrcDelegation.shares - amountWithPrecision
        
        // Assert exact shares changes  
        expect(afterSrcDelegation.shares).to.equal(
            expectedSrcShares,
            'Source validator delegation shares should match expected value'
        )
        
        // For destination validator, shares should increase by the amount with precision
        expect(afterDstDelegation.shares).to.equal(
            beforeDstDelegation.shares + amountWithPrecision,
            'Destination validator delegation shares should increase by redelegated amount with precision'
        )

        // Verify denomination consistency
        expect(afterSrcDelegation.balance.denom).to.equal(beforeSrcDelegation.balance.denom)
        expect(afterDstDelegation.balance.denom).to.equal(beforeDstDelegation.balance.denom)

        // 6) query redelegation state after
        const afterRaw = await staking.redelegation(signer.address, srcValBech32, dstValBech32)
        const afterR = formatRedelegation(afterRaw)
        console.log('After redelegation:', afterR)
        const entriesAfter = afterR.entries.length

        // Assert that a new redelegation entry was created
        expect(entriesAfter).to.equal(
            entriesBefore + 1,
            'Number of redelegation entries should increase by 1'
        )
        // Assert that the latest entry initialBalance matches the redelegated amount
        expect(afterR.delegatorAddress).to.equal(signerBech32)
        expect(afterR.validatorSrcAddress).to.equal(srcValBech32)
        expect(afterR.validatorDstAddress).to.equal(dstValBech32)
        expect(afterR.entries[0].initialBalance).to.equal(
            amount,
            'Redelegation entry initialBalance should match redelegated amount'
        )
        expect(afterR.entries[0].sharesDst).to.equal(
            amountWithPrecision,
            'Redelegation entry sharesDst should match redelegated amount with precision'
        )

        const pageRequest = {key: '0x', offset: 0, limit: 10, countTotal: true, reverse: false}
        const [responses, _] = await staking.redelegations(
            signer.address,
            srcValBech32,
            dstValBech32,
            pageRequest
        )
        expect(responses.length).to.be.gte(1, 'redelegations() should return at least one response')
        // check first response matches singular result
        const response = responses[0]
        const redelegation = response[0]
        const entries = response[1]

        // the 'redelegation' field is a Redelegation struct
        expect(redelegation.delegatorAddress).to.equal(afterR.delegatorAddress)
        expect(redelegation.validatorSrcAddress).to.equal(afterR.validatorSrcAddress)
        expect(redelegation.validatorDstAddress).to.equal(afterR.validatorDstAddress)
        // the 'entries' field is RedelegationEntryResponse[]
        expect(entries.length).to.equal(entriesAfter)
        const entryResp = entries[0]
        // check RedelegationEntryResponse.redelegationEntry.initialBalance
        expect(
            BigInt(entryResp.redelegationEntry.initialBalance.toString())
        ).to.equal(
            afterR.entries[0].initialBalance,
            'list entry initialBalance should match singular result'
        )
        // check RedelegationEntryResponse.balance
        expect(
            BigInt(entryResp.balance.toString())
        ).to.equal(
            afterR.entries[0].initialBalance,
            'list entry balance should match singular result'
        )
    })
})