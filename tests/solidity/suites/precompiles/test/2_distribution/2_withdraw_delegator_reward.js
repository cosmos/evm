const { expect } = require('chai');
const hre = require('hardhat');

describe('Distribution – withdraw delegator reward', function () {
    const STAKING_ADDRESS = '0x0000000000000000000000000000000000000800'
    const DIST_ADDRESS = '0x0000000000000000000000000000000000000801';
    const gasLimit = 1_000_000;

    let staking, distribution, signer;

    before(async () => {
        [signer] = await hre.ethers.getSigners();

        staking = await hre.ethers.getContractAt('StakingI', STAKING_ADDRESS)
        distribution = await hre.ethers.getContractAt('DistributionI', DIST_ADDRESS);
    });

    it('should withdraw rewards and emit WithdrawDelegatorReward event', async function () {
        const valBech32 = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql';
        const valHex = '0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E';
        const stakeAmountBn = hre.ethers.parseEther('0.001')   // BigNumber
        const stakeAmount = BigInt(stakeAmountBn.toString())

        // Delegate to the validator first
        const delegateTx = await staking
            .connect(signer)
            .delegate(signer.address, valBech32, stakeAmount, {gasLimit: gasLimit})
        const delegateReceipt = await delegateTx.wait(2)
        console.log('Delegate tx hash:', delegateReceipt.hash, 'gas used:', delegateReceipt.gasUsed.toString())

        // Sleep to ensure rewards are available
        console.log('Waiting for rewards to accumulate... (5s)');
        await new Promise(resolve => setTimeout(resolve, 5000)); // wait 5 seconds

        // Query accumulated rewards before withdrawal
        const result = await distribution.delegationRewards(signer.address, valBech32);
        const currentReward = result[0];

        const tx = await distribution
            .connect(signer)
            .withdrawDelegatorRewards(signer.address, valBech32, {gasLimit});
        const receipt = await tx.wait(2);
        console.log('WithdrawDelegatorRewards tx hash:', receipt.hash);

        // Check events
        const evt = receipt.logs
            .map(log => { try { return distribution.interface.parseLog(log); } catch { return null; } })
            .find(e => e && e.name === 'WithdrawDelegatorReward');
        expect(evt, 'WithdrawDelegatorReward event must be emitted').to.exist;
        expect(evt.args.delegatorAddress).to.equal(signer.address);
        expect(evt.args.validatorAddress).to.equal(valHex);
        expect(evt.args.amount).to.be.a('bigint');
        expect(evt.args.amount).to.be.greaterThan(currentReward.amount, 'Withdrawn amount should be greater than zero');

        // Check state after withdrawal
        const afterResult = await distribution.delegationRewards(signer.address, valBech32);
        const afterReward = afterResult[0];
        // afterReward should be less than currentReward
        expect(afterReward.amount).to.be.lessThan(currentReward.amount, 'Rewards should be reduced')
    });
});
