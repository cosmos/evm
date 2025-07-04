const hre = require('hardhat');
const {expect} = require('chai');

describe('Bank', function () {
    it('query account balances', async function () {
        const bank = await hre.ethers.getContractAt(
            'IBank',
            '0x0000000000000000000000000000000000000804'
        );
        const [signer] = await hre.ethers.getSigners();
        const balances = await bank
            .getFunction('balances')
            .staticCall(signer.address);
        console.log('Balances:', balances);
        expect(balances.length).to.be.greaterThan(0);
        expect(balances[0][0]).to.be.eq('0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE')
        expect(balances[0].amount).to.be.a('bigint');
    });

    it('query total supply', async function () {
        const bank = await hre.ethers.getContractAt(
            'IBank',
            '0x0000000000000000000000000000000000000804'
        );
        const supply = await bank.getFunction('totalSupply').staticCall();
        console.log('Total supply length:', supply.length);
        expect(supply.length).to.be.greaterThan(0);
    });

    it('query supply of WEVMOS', async function () {
        const bank = await hre.ethers.getContractAt(
            'IBank',
            '0x0000000000000000000000000000000000000804'
        );
        const wevmos = '0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE';
        const supply = await bank.getFunction('supplyOf').staticCall(wevmos);
        console.log('Native token supply:', supply.toString());
        expect(supply).to.be.a('bigint');
    });
});