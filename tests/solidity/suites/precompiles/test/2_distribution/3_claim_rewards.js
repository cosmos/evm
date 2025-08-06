const { expect } = require('chai');
const hre = require('hardhat');
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common');

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
        // This address is a current withdraw address for the signer. Check 1_set_withdraw_address.js test for more details.
        const newWithdrawAddress = '0x498B5AeC5D439b733dC2F58AB489783A23FB26dA';

        // Check user balance before claiming rewards
        const balanceBefore = await hre.ethers.provider.getBalance(newWithdrawAddress);
        console.log('User balance before claiming:', balanceBefore.toString());

        const tx = await distribution
            .connect(signer)
            .claimRewards(signer.address, 5, { gasLimit: GAS_LIMIT });
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)
        console.log('ClaimRewards tx hash:', receipt.hash, 'gas used:', receipt.gasUsed.toString());

        // Check user balance after claiming rewards
        const balanceAfter = await hre.ethers.provider.getBalance(newWithdrawAddress);
        console.log('User balance after claiming:', balanceAfter.toString());

        const evt = findEvent(receipt.logs, distribution.interface, 'ClaimRewards');
        expect(evt, 'ClaimRewards event should be emitted').to.exist;
        expect(evt.args.delegatorAddress).to.equal(signer.address);
        expect(evt.args.amount).to.be.a('bigint');
        console.log('totalRewards claimed:', evt.args.amount);

        // Validate balance increase (accounting for gas costs)
        const gasUsed = receipt.gasUsed * receipt.gasPrice;
        const expectedMinBalance = balanceBefore - gasUsed + evt.args.amount;
        expect(balanceAfter).to.be.gte(expectedMinBalance, 'User balance should increase by claimed rewards minus gas costs');
        console.log('finished balance checks');

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
