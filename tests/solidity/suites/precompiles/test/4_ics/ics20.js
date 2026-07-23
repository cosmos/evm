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
            const revertData = getRevertData(error);
            expect(revertData.slice(0, 10)).to.equal('0xff11f3e3');

            const parsed = ics20.interface.parseError(revertData);
            expect(parsed.name).to.equal('IBCChannelNotFound');
            expect(parsed.signature).to.equal('IBCChannelNotFound()');
            expect(parsed.args).to.have.length(0);
            expect(parsed.name).not.to.equal('MsgServerFailed');
            expect(parsed.name).not.to.equal('QueryFailed');
            expect(parsed.name).not.to.equal('UnmappedCosmosError');
        }
    });

    it('decodes a mapped IBC channel error without a generic fallback', async function () {
        try {
            await ics20.transfer.estimateGas(
                'transfer',
                'channel-999',
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
            const data = getRevertData(error);
            const parsed = ics20.interface.parseError(data);
            expect(parsed.name).to.equal('IBCChannelNotFound');
            expect(parsed.args).to.have.length(0);
            expect(['MsgServerFailed', 'QueryFailed', 'UnmappedCosmosError'])
                .not.to.include(parsed.name);
            expect(data.slice(0, 10))
                .to.equal(ics20.interface.getError('IBCChannelNotFound').selector);
        }
    });

    it('decodes an invalid source port as a precompile-native error', async function () {
        try {
            await ics20.transfer.estimateGas(
                'invalid/port',
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
            const data = getRevertData(error);
            const parsed = ics20.interface.parseError(data);
            expect(parsed.name).to.equal('InvalidSourcePort');
            expect(parsed.args.callMethod).to.equal('transfer');
            expect(parsed.args.reason).to.equal('invalid source port');
            expect(['MsgServerFailed', 'QueryFailed', 'UnmappedCosmosError'])
                .not.to.include(parsed.name);
        }
    });

    it('decodes an invalid source channel as a precompile-native error', async function () {
        try {
            await ics20.transfer.estimateGas(
                'transfer',
                'invalid/channel',
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
            const data = getRevertData(error);
            const parsed = ics20.interface.parseError(data);
            expect(parsed.name).to.equal('InvalidSourceChannel');
            expect(parsed.args.callMethod).to.equal('transfer');
            expect(parsed.args.reason).to.equal('invalid source channel');
            expect(['MsgServerFailed', 'QueryFailed', 'UnmappedCosmosError'])
                .not.to.include(parsed.name);
        }
    });
});
