const {expect} = require('chai');
const hre = require('hardhat');
const { BECH32_PRECOMPILE_ADDRESS, getRevertData } = require('../common');

describe('Bech32', function () {
    let bech32;

    before(async function () {
        bech32 = await hre.ethers.getContractAt('Bech32I', BECH32_PRECOMPILE_ADDRESS);
    });

    it('hex to bech32 and back', async function () {
        const [signer] = await hre.ethers.getSigners();
        const bech32Addr = await bech32.getFunction('hexToBech32').staticCall(
            signer.address,
            'cosmos'
        );
        const hexAddr = await bech32.getFunction('bech32ToHex').staticCall(bech32Addr);
        console.log('Bech32:', bech32Addr, 'Hex:', hexAddr);
        expect(hexAddr).to.equal(signer.address);
    });

    it('decodes invalid bech32 custom error data', async function () {
        try {
            await bech32.getFunction('bech32ToHex').staticCall('not-a-bech32-address');
            expect.fail('bech32ToHex should have reverted');
        } catch (error) {
            const parsed = bech32.interface.parseError(getRevertData(error));
            expect(parsed.name).to.equal('InvalidAddress');
            expect(String(parsed.args[0])).to.include('not-a-bech32-address');
        }
    });
});
