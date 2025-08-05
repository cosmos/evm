const { expect } = require('chai');
const hre = require('hardhat');
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common');

describe('Distribution – fund community pool', function () {
    const DIST_ADDRESS = '0x0000000000000000000000000000000000000801';
    const GAS_LIMIT = 1_000_000;

    let distribution, signer;

    before(async () => {
        [signer] = await hre.ethers.getSigners();
        distribution = await hre.ethers.getContractAt('DistributionI', DIST_ADDRESS);
    });

    it('funds the community pool and emits FundCommunityPool event', async function () {
        const coin = { denom: 'atest', amount: hre.ethers.parseEther('0.01') };

        const beforePool = await distribution.communityPool();
        
        // Check user balance before funding
        const balanceBefore = await hre.ethers.provider.getBalance(signer.address);
        console.log('User balance before funding:', balanceBefore.toString());

        const tx = await distribution
            .connect(signer)
            .fundCommunityPool(signer.address, [coin], { gasLimit: GAS_LIMIT });
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);
        console.log('FundCommunityPool tx hash:', receipt.hash);

        // Check user balance after funding
        const balanceAfter = await hre.ethers.provider.getBalance(signer.address);
        console.log('User balance after funding:', balanceAfter.toString());

        const evt = findEvent(receipt.logs, distribution.interface, 'FundCommunityPool');
        expect(evt, 'FundCommunityPool event must be emitted').to.exist;
        expect(evt.args.depositor).to.equal(signer.address);
        expect(evt.args.denom).to.equal(coin.denom);
        expect(evt.args.amount.toString()).to.equal(coin.amount.toString());

        // Validate user balance decreased by funding amount plus gas costs
        const gasUsed = receipt.gasUsed * receipt.gasPrice;
        const expectedBalance = balanceBefore - BigInt(coin.amount.toString()) - gasUsed;
        expect(balanceAfter).to.equal(expectedBalance, 'User balance should decrease by funding amount plus gas costs');
        console.log('finished balance checks');

        const afterPool = await distribution.communityPool();
        const beforeAmt = beforePool.find(c => c.denom === coin.denom);
        const afterAmt = afterPool.find(c => c.denom === coin.denom);
        console.log('Community pool before funding:', beforeAmt.amount);
        console.log('Community pool after funding:', afterAmt.amount);
        const start = beforeAmt ? BigInt(beforeAmt.amount.toString()) : 0n;
        const end = afterAmt ? BigInt(afterAmt.amount.toString()) : 0n;
        // Community pool is continuously increasing by collecting fee, so end should be greater than or equal to start + funded amount
        expect(end).to.gte(start + coin.amount, 'Community pool should increase by funded amount');
    });
});

