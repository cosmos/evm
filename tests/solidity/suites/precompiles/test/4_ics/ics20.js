const { expect } = require('chai');
const hre = require('hardhat');
const { ICS20_PRECOMPILE_ADDRESS, getRevertData } = require('../common');

describe('ICS20 Precompile', function () {
    let ics20, signer;

    before(async function () {
        [signer] = await hre.ethers.getSigners();
        ics20 = await hre.ethers.getContractAt('ICS20I', ICS20_PRECOMPILE_ADDRESS);
    });

    it('decodes transfer custom error data', async function () {
        try {
            await ics20.transfer.estimateGas(
                'bad-port',
                'channel-0',
                'atest',
                1,
                signer.address,
                'cosmos1cml96vmptgw99syqrrz8az79xer2pcgp95srxm',
                { revisionNumber: 0, revisionHeight: 0 },
                0,
                ''
            );
            expect.fail('transfer should have reverted');
        } catch (error) {
            const parsed = ics20.interface.parseError(getRevertData(error));
            expect(parsed.name).to.equal('MsgServerFailed');
            expect(parsed.args[0]).to.equal('transfer');
        }
    });
});
