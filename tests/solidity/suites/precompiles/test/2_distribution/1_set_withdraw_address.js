const {expect} = require('chai');
const hre = require('hardhat');
const { findEvent, waitWithTimeout, DEFAULT_TIMEOUT, RETRY_DELAY_FUNC} = require('../common');

describe('Distribution – set withdraw address', function () {
    this.timeout(120000); // 2 minutes timeout
    const DIST_ADDRESS = '0x0000000000000000000000000000000000000801';
    const GAS_LIMIT = 1_000_000;

    let distribution, signer;

    before(async () => {
        [signer] = await hre.ethers.getSigners();
        distribution = await hre.ethers.getContractAt('DistributionI', DIST_ADDRESS);
    });

    it('should set withdraw address and emit SetWithdrawerAddress event', async function () {
        // Use epix prefix instead of cosmos for EpixChain
        // 0x498B5AeC5D439b733dC2F58AB489783A23FB26dA converts to epix1fx944mzagwdhx0wz7k9tfztc8g3lkfk65fhh8v
        const newWithdrawAddress = 'epix1fx944mzagwdhx0wz7k9tfztc8g3lkfk65fhh8v';
        const tx = await distribution
            .connect(signer)
            .setWithdrawAddress(signer.address, newWithdrawAddress, {gasLimit: GAS_LIMIT});
        const receipt = await waitWithTimeout(tx, DEFAULT_TIMEOUT, RETRY_DELAY_FUNC);
        console.log('SetWithdrawAddress tx hash:', receipt.hash);

        const evt = findEvent(receipt.logs, distribution.interface, 'SetWithdrawerAddress');
        expect(evt, 'SetWithdrawerAddress event must be emitted').to.exist;
        expect(evt.args.caller).to.equal(signer.address);
        expect(evt.args.withdrawerAddress).to.equal(newWithdrawAddress);

        const withdrawer = await distribution.delegatorWithdrawAddress(signer.address);
        console.log('Withdraw address:', withdrawer);
        expect(withdrawer).to.equal(newWithdrawAddress);
    });
});