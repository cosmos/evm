const { expect } = require('chai');
const hre = require('hardhat');
const { WERC20_ADDRESS, DEFAULT_GAS_LIMIT, findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common');

describe('WERC20 – deposit and withdraw', function () {
    const GAS_LIMIT = DEFAULT_GAS_LIMIT;
    
    let werc20, signer;

    before(async function () {
        [signer] = await hre.ethers.getSigners();
        werc20 = await hre.ethers.getContractAt('IWERC20', WERC20_ADDRESS);
    });

    describe('Deposit functionality', function () {
        it('deposits native tokens successfully', async function () {
            const depositAmount = hre.ethers.parseEther('1.0');
            
            // Check balances before deposit
            const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceBefore = await hre.ethers.provider.getBalance(WERC20_ADDRESS);

            console.log('Depositing', hre.ethers.formatEther(depositAmount), 'tokens');
            
            const tx = await werc20.deposit({
                value: depositAmount,
                gasLimit: GAS_LIMIT
            });
            const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);

            // Check balances after deposit
            const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceAfter = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
            
            console.log('Deposit transaction hash:', receipt.hash);
            console.log('Gas used:', receipt.gasUsed.toString());
            
            // Check that transaction was successful
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
            const gasFee = receipt.gasUsed * receipt.gasPrice;

            // Verify balance changes
            expect(contractBalanceAfter).to.equal(contractBalanceBefore);
            expect(signerBalanceAfter).to.equal(signerBalanceBefore - gasFee);
            
            // Look for Deposit event
            const parsed = findEvent(receipt.logs, werc20.interface, 'Deposit');
            console.log('Deposit event:', parsed.args);
            expect(parsed.args.dst).to.equal(signer.address);
            expect(parsed.args.wad).to.equal(depositAmount);
        });

        it('deposits with different amounts', async function () {
            const amounts = [
                hre.ethers.parseEther('0.1'),   // Small amount
                hre.ethers.parseEther('5.0'),   // Medium amount
                hre.ethers.parseEther('10.0'),  // Large amount
            ];

            for (const amount of amounts) {
                // Check balances before deposit
                const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address);
                const contractBalanceBefore = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
                
                console.log('Depositing', hre.ethers.formatEther(amount), 'tokens');
                
                const tx = await werc20.deposit({
                    value: amount,
                    gasLimit: GAS_LIMIT
                });
                const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);

                // Check balances after deposit
                const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address);
                const contractBalanceAfter = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
                
                console.log('Gas used for', hre.ethers.formatEther(amount), 'deposit:', receipt.gasUsed.toString());
                
                expect(receipt.status).to.equal(1);
                expect(receipt.gasUsed).to.be.greaterThan(0);
                const gasFee = receipt.gasUsed * receipt.gasPrice;
                
                // Verify balance changes
                expect(contractBalanceAfter).to.equal(contractBalanceBefore);
                expect(signerBalanceAfter).to.equal(signerBalanceBefore - gasFee);
            }
        });

        it('deposits via fallback function', async function () {
            const depositAmount = hre.ethers.parseEther('0.5');
            
            // Check balances before deposit
            const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceBefore = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
            
            console.log('Depositing via fallback function');
            
            // Send ETH directly to the contract (should trigger fallback/receive)
            const tx = await signer.sendTransaction({
                to: WERC20_ADDRESS,
                value: depositAmount,
                gasLimit: GAS_LIMIT
            });
            const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);

            // Check balances after deposit
            const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceAfter = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
            
            console.log('Fallback deposit gas used:', receipt.gasUsed.toString());
            
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
            const gasFee = receipt.gasUsed * receipt.gasPrice;
            
            // Verify balance changes
            expect(contractBalanceAfter).to.equal(contractBalanceBefore);
            expect(signerBalanceAfter).to.equal(signerBalanceBefore - gasFee);
        });
    });

    describe('Withdraw functionality', function () {
        it('withdraws tokens successfully', async function () {
            const withdrawAmount = hre.ethers.parseEther('1.0');
            
            // Check balances before withdrawal
            const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceBefore = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
            
            console.log('Withdrawing', hre.ethers.formatEther(withdrawAmount), 'tokens');
            
            const tx = await werc20.withdraw(withdrawAmount, {
                gasLimit: GAS_LIMIT
            });
            const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);

            // Check balances after withdrawal
            const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceAfter = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
            
            console.log('Withdraw transaction hash:', receipt.hash);
            console.log('Gas used:', receipt.gasUsed.toString());
            
            // Check that transaction was successful
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
            const gasFee = receipt.gasUsed * receipt.gasPrice;
            
            // Verify balance changes
            expect(contractBalanceAfter).to.equal(contractBalanceBefore);
            expect(signerBalanceAfter).to.equal(signerBalanceBefore - gasFee);
            
            // Look for Withdrawal event
            const parsed = findEvent(receipt.logs, werc20.interface, 'Withdrawal');
            console.log('Withdrawal event:', parsed.args);
            expect(parsed.args.src).to.equal(signer.address);
            expect(parsed.args.wad).to.equal(withdrawAmount);
        });

        it('withdraws different amounts', async function () {
            const amounts = [
                hre.ethers.parseEther('0.05'),  // Very small amount
                hre.ethers.parseEther('0.5'),   // Small amount
                hre.ethers.parseEther('3.0'),   // Medium amount
                hre.ethers.parseEther('7.5'),   // Large amount
            ];

            for (const amount of amounts) {
                // Check balances before withdrawal
                const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address);
                const contractBalanceBefore = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
                
                console.log('Withdrawing', hre.ethers.formatEther(amount), 'tokens');
                
                const tx = await werc20.withdraw(amount, {
                    gasLimit: GAS_LIMIT
                });
                const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);

                // Check balances after withdrawal
                const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address);
                const contractBalanceAfter = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
                
                console.log('Gas used for', hre.ethers.formatEther(amount), 'withdrawal:', receipt.gasUsed.toString());
                
                expect(receipt.status).to.equal(1);
                expect(receipt.gasUsed).to.be.greaterThan(0);
                const gasFee = receipt.gasUsed * receipt.gasPrice;
                
                // Verify balance changes
                expect(contractBalanceAfter).to.equal(contractBalanceBefore);
                expect(signerBalanceAfter).to.equal(signerBalanceBefore - gasFee);
            }
        });

        it('withdraws zero amount (edge case)', async function () {
            const zeroAmount = hre.ethers.parseEther('0');
            
            // Check balances before withdrawal
            const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceBefore = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
            
            console.log('Withdrawing zero amount');
            
            const tx = await werc20.withdraw(zeroAmount, {
                gasLimit: GAS_LIMIT
            });
            const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);

            // Check balances after withdrawal
            const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address);
            const contractBalanceAfter = await hre.ethers.provider.getBalance(WERC20_ADDRESS);
            
            console.log('Gas used for zero withdrawal:', receipt.gasUsed.toString());
            
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
            
            // Verify balance changes (should be no change for zero withdrawal except gas fees)
            const gasFee = receipt.gasUsed * receipt.gasPrice;
            expect(contractBalanceAfter).to.equal(contractBalanceBefore);
            expect(signerBalanceAfter).to.equal(signerBalanceBefore - gasFee);
        });
    });
});