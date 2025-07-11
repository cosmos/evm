const { expect } = require('chai');
const hre = require('hardhat');

/**
 * Parse the raw return from delegationTotalRewards into structured objects.
 * @param {[DelegationDelegatorReward[], DecCoin[]]} raw
 * @returns {{ delegationRewards: { validatorAddress: string, rewards: { denom: string, amount: BigInt, precision: number }[] }[], totalRewards: { denom: string, amount: BigInt, precision: number }[] }}
 */
function formatTotalRewards([delegationRewardsRaw, totalRaw]) {
    const delegationRewards = delegationRewardsRaw.map(([validatorAddress, decCoins]) => ({
        validatorAddress,
        rewards: decCoins.map(([denom, amountBn, precisionBn]) => ({
            denom,
            amount: BigInt(amountBn.toString()),
            precision: Number(precisionBn.toString()),
        })),
    }))

    const totalRewards = totalRaw.map(([denom, amountBn, precisionBn]) => ({
        denom,
        amount: BigInt(amountBn.toString()),
        precision: Number(precisionBn.toString()),
    }))

    return { delegationRewards, totalRewards }
}

describe('DistributionI – claimRewards', function () {
    const DISTRIBUTION_ADDRESS = '0x0000000000000000000000000000000000000801';
    const GAS_LIMIT = 1_000_000;

    let distribution, signer;

    before(async () => {
        [signer] = await hre.ethers.getSigners();
        distribution = await hre.ethers.getContractAt('DistributionI', DISTRIBUTION_ADDRESS);
    });

    it('should claim rewards from at most one validator', async function () {
        // Query delegation total rewards
        const delegatorAddress = await signer.getAddress();
        const rawRewards = await distribution.delegationTotalRewards(delegatorAddress);
        const { delegationRewards, totalRewards } = formatTotalRewards(rawRewards)
        console.log('Parsed delegationRewards:', delegationRewards)
        console.log('Parsed totalRewards:', totalRewards)

        const tx = await distribution
            .connect(signer)
            .claimRewards(signer.address, 5, { gasLimit: GAS_LIMIT });
        const receipt = await tx.wait(2);
        console.log('ClaimRewards tx hash:', receipt.hash, 'gas used:', receipt.gasUsed.toString());

        const evt = receipt.logs
            .map(log => {
                try { return distribution.interface.parseLog(log); } catch { return null; }
            })
            .find(e => e && e.name === 'ClaimRewards');
        expect(evt, 'ClaimRewards event should be emitted').to.exist;
        expect(evt.args.delegatorAddress).to.equal(signer.address);
        expect(evt.args.amount).to.be.a('bigint');
        console.log('totalRewards claimed:', evt.args.amount);

        // 3) query total rewards after claim, re-parse
        const postRaw = await distribution.delegationTotalRewards(delegatorAddress)
        const { delegationRewards: postDelegation, totalRewards: postTotal } = formatTotalRewards(postRaw)

        console.log('Parsed delegationRewards after claim:', postDelegation)
        console.log('Parsed totalRewards after claim:', postTotal)

        // assert that totalRewards decreased by claimed amount (if only one denom)
        const beforeTotal = totalRewards.reduce((acc, c) => acc + c.amount, 0n)
        const afterTotal  = postTotal.reduce((acc, c) => acc + c.amount, 0n)
        expect(afterTotal).to.lessThan(beforeTotal)
    });
});
