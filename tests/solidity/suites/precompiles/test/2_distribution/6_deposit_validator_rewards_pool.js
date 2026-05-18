const { expect } = require('chai');
const hre = require('hardhat');
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common');

describe('Distribution – deposit validator rewards pool', function () {
    const DIST_ADDRESS = '0x0000000000000000000000000000000000000801';
    const GAS_LIMIT = 1_000_000;
    const VAL_BECH32 = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql';
    const VAL_HEX = '0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E';

    let distribution, signer;

    before(async () => {
        [signer] = await hre.ethers.getSigners();
        distribution = await hre.ethers.getContractAt('DistributionI', DIST_ADDRESS);
    });

    it('deposits rewards and emits DepositValidatorRewardsPool event', async function () {
        const coin = { denom: 'atest', amount: hre.ethers.parseEther('0.1') };

        const beforeRewards = await distribution.validatorOutstandingRewards(VAL_BECH32);
        const beforeCoin = beforeRewards.find(c => c.denom === coin.denom);
        const start = beforeCoin ? BigInt(beforeCoin.amount.toString()) : 0n;

        const balanceBefore = await hre.ethers.provider.getBalance(signer.address);
        console.log('User balance before deposit:', balanceBefore.toString());

        const tx = await distribution
            .connect(signer)
            .depositValidatorRewardsPool(signer.address, VAL_BECH32, [coin], { gasLimit: GAS_LIMIT });
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);
        console.log('DepositValidatorRewardsPool tx hash:', receipt.hash);

        const balanceAfter = await hre.ethers.provider.getBalance(signer.address);
        console.log('User balance after deposit:', balanceAfter.toString());

        const evt = findEvent(receipt.logs, distribution.interface, 'DepositValidatorRewardsPool');
        expect(evt, 'DepositValidatorRewardsPool event must be emitted').to.exist;
        expect(evt.args.depositor).to.equal(signer.address);
        expect(evt.args.validatorAddress).to.equal(VAL_HEX);
        expect(evt.args.denom).to.equal(coin.denom);
        expect(evt.args.amount.toString()).to.equal(coin.amount.toString());

        const gasUsed = receipt.gasUsed * receipt.gasPrice;
        const expectedBalance = balanceBefore - BigInt(coin.amount.toString()) - gasUsed;
        expect(balanceAfter).to.equal(expectedBalance, 'User balance should decrease by deposit amount plus gas costs');
        console.log('finished balance checks');

        const afterRewards = await distribution.validatorOutstandingRewards(VAL_BECH32);
        const afterCoin = afterRewards.find(c => c.denom === coin.denom);
        const end = afterCoin ? BigInt(afterCoin.amount.toString()) : 0n;
        expect(end).to.gte(start + BigInt(coin.amount.toString()));
    });
});
