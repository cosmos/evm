const {expect} = require('chai');
const hre = require('hardhat');

describe('Bech32', function () {
    it('hex to bech32 and back', async function () {
        const bech32 = await hre.ethers.getContractAt(
            'Bech32I',
            '0x0000000000000000000000000000000000000400'
        );

        const [signer] = await hre.ethers.getSigners();
        const bech32Addr = await bech32.getFunction('hexToBech32').staticCall(
            signer.address,
            'cosmos'
        );
        const hexAddr = await bech32.getFunction('bech32ToHex').staticCall(bech32Addr);
        console.log('Bech32:', bech32Addr, 'Hex:', hexAddr);
        expect(hexAddr).to.equal(signer.address);
    });
});