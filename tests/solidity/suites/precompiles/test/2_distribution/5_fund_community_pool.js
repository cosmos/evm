const { expect } = require('chai');
const hre = require('hardhat');

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

        const tx = await distribution
            .connect(signer)
            .fundCommunityPool(signer.address, [coin], { gasLimit: GAS_LIMIT });
        const receipt = await tx.wait(2);
        console.log('FundCommunityPool tx hash:', receipt.hash);

        const evt = receipt.logs
            .map(log => {
                try { return distribution.interface.parseLog(log); } catch { return null; }
            })
            .find(e => e && e.name === 'FundCommunityPool');
        expect(evt, 'FundCommunityPool event must be emitted').to.exist;
        expect(evt.args.depositor).to.equal(signer.address);
        expect(evt.args.denom).to.equal(coin.denom);
        expect(evt.args.amount.toString()).to.equal(coin.amount.toString());

        const afterPool = await distribution.communityPool();
        const beforeAmt = beforePool.find(c => c.denom === coin.denom);
        const afterAmt = afterPool.find(c => c.denom === coin.denom);
        const start = beforeAmt ? BigInt(beforeAmt.amount.toString()) : 0n;
        const end = afterAmt ? BigInt(afterAmt.amount.toString()) : 0n;
        expect(end).to.gte(start + BigInt(coin.amount.toString()));
    });
});

