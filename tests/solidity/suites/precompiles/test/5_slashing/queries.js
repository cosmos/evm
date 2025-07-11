const { expect } = require('chai');
const hre = require('hardhat');

describe('Slashing – query methods', function () {
    const SLASHING_ADDRESS = '0x0000000000000000000000000000000000000806';
    const CONS_ADDR = '0x020a0f48a2f4ce0f0cA6debF71DB83474dD717D0' // address derived from ed25519 pubkey which is placed in ~/.evmd/config/priv_validator_key.json
    let slashing;

    before(async function () {
        slashing = await hre.ethers.getContractAt('ISlashing', SLASHING_ADDRESS);
    });

    it('getSigningInfos returns list of signing info', async function () {
        const pageReq = { key: '0x', offset: 0, limit: 10, countTotal: true, reverse: false };
        const [infos, pageResponse] = await slashing.getSigningInfos(pageReq);
        console.log('Signing infos:', infos, 'Page:', pageResponse);
        expect(infos.length).to.be.greaterThan(0);
        expect(pageResponse.total).to.be.a('bigint').and.to.be.greaterThan(0n);
        const info = infos[0];
        expect(info.validatorAddress).to.equal(CONS_ADDR);
        expect(info.startHeight).to.be.a('bigint');
        expect(info.indexOffset).to.be.a('bigint');
        expect(info.tombstoned).to.be.a('boolean');
        expect(info.missedBlocksCounter).to.be.a('bigint');
    });

    it('getSigningInfo returns info for a validator', async function () {
        const info = await slashing.getSigningInfo(CONS_ADDR);
        console.log('Signing info:', info);
        expect(info.validatorAddress).to.equal(CONS_ADDR);
        expect(info.startHeight).to.be.a('bigint');
    });

    it('getParams returns slashing module params', async function () {
        const params = await slashing.getParams();
        console.log('Params:', params);
        expect(params.signedBlocksWindow).to.be.a('bigint');
        expect(params.minSignedPerWindow.value).to.be.a('bigint');
        expect(params.downtimeJailDuration).to.be.a('bigint');
        expect(params.slashFractionDoubleSign.value).to.be.a('bigint');
        expect(params.slashFractionDowntime.value).to.be.a('bigint');
    });
});
