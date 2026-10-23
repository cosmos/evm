const { expect } = require('chai');
const hre = require('hardhat');
const { ICS02_PRECOMPILE_ADDRESS, getRevertData } = require('../common');

describe('ICS02 Precompile', function () {
    let ics02;

    before(async function () {
        ics02 = await hre.ethers.getContractAt('ICS02I', ICS02_PRECOMPILE_ADDRESS);
    });

    it('decodes invalid client ID custom error data', async function () {
        try {
            await ics02.updateClient.estimateGas('', '0x');
            expect.fail('updateClient should have reverted');
        } catch (error) {
            const parsed = ics02.interface.parseError(getRevertData(error));
            expect(parsed.name).to.equal('InvalidClientID');
            expect(parsed.args[0]).to.equal('');
        }
    });
});
