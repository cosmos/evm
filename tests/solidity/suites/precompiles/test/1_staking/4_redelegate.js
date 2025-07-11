const {expect} = require('chai')
const hre = require('hardhat')

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

        // 1) query current delegation to source validator
        const beforeDelegation = await staking.delegation(signer.address, srcValBech32)
        const amount = beforeDelegation.balance.amount
        console.log('Current delegation to srcVal:', amount.toString())

        // 2) query redelegation entries before
        const beforeRaw = await staking.redelegation(signer.address, srcValBech32, dstValBech32)
        const beforeR = formatRedelegation(beforeRaw)
        const entriesBefore = beforeR.entries.length

        // 3) send the redelegate transaction
        const tx = await staking
            .connect(signer)
            .redelegate(signer.address, srcValBech32, dstValBech32, amount, {gasLimit: GAS_LIMIT})
        const receipt = await tx.wait(2)
        console.log('Redelegate tx hash:', tx.hash, 'gas used:', receipt.gasUsed.toString())

        // 4) parse and assert the Redelegate event
        const redelegateEvt = receipt.logs
            .map(log => {
                try {
                    return staking.interface.parseLog(log)
                } catch {
                    return null
                }
            })
            .find(evt => evt && evt.name === 'Redelegate')
        expect(redelegateEvt, 'Redelegate event should be emitted').to.exist
        expect(redelegateEvt.args.delegatorAddress).to.equal(signer.address)
        expect(redelegateEvt.args.validatorSrcAddress).to.equal(srcValHex)
        expect(redelegateEvt.args.validatorDstAddress).to.equal(dstValHex)
        expect(redelegateEvt.args.amount).to.equal(amount)
        const completionTime = BigInt(redelegateEvt.args.completionTime.toString())
        expect(completionTime > 0n, 'completionTime should be positive').to.be.true

        // 5) query redelegation state after
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
            BigInt(amount.toString()),
            'Redelegation entry initialBalance should match redelegated amount'
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