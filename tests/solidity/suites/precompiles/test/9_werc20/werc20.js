const { expect } = require('chai');
const hre = require('hardhat');

describe('WERC20 – deposit and withdraw', function () {
    // Using a placeholder address for WERC20 - in reality this would be dynamic
    // based on the actual ERC20 token pair configuration
    const WERC20_ADDRESS = '0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE';
    const GAS_LIMIT = 1_000_000;
    
    let werc20, signer;

    before(async function () {
        [signer] = await hre.ethers.getSigners();
        werc20 = await hre.ethers.getContractAt('IWERC20', WERC20_ADDRESS);
    });

    describe('Deposit functionality', function () {
        it('deposits native tokens successfully', async function () {
            const depositAmount = hre.ethers.parseEther('1.0');
            
            console.log('Depositing', hre.ethers.formatEther(depositAmount), 'tokens');
            
            const tx = await werc20.deposit({
                value: depositAmount,
                gasLimit: GAS_LIMIT
            });
            const receipt = await tx.wait();
            
            console.log('Deposit transaction hash:', receipt.hash);
            console.log('Gas used:', receipt.gasUsed.toString());
            
            // Check that transaction was successful
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
            
            // Look for Deposit event
            const depositEvent = receipt.logs.find(log => {
                try {
                    const parsed = werc20.interface.parseLog(log);
                    return parsed && parsed.name === 'Deposit';
                } catch {
                    return false;
                }
            });
            
            const parsed = werc20.interface.parseLog(depositEvent);
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
                console.log('Depositing', hre.ethers.formatEther(amount), 'tokens');
                
                const tx = await werc20.deposit({
                    value: amount,
                    gasLimit: GAS_LIMIT
                });
                const receipt = await tx.wait();
                
                console.log('Gas used for', hre.ethers.formatEther(amount), 'deposit:', receipt.gasUsed.toString());
                
                expect(receipt.status).to.equal(1);
                expect(receipt.gasUsed).to.be.greaterThan(0);
            }
        });

        it('deposits via fallback function', async function () {
            const depositAmount = hre.ethers.parseEther('0.5');
            
            console.log('Depositing via fallback function');
            
            // Send ETH directly to the contract (should trigger fallback/receive)
            const tx = await signer.sendTransaction({
                to: WERC20_ADDRESS,
                value: depositAmount,
                gasLimit: GAS_LIMIT
            });
            const receipt = await tx.wait();
            
            console.log('Fallback deposit gas used:', receipt.gasUsed.toString());
            
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
        });
    });

    describe('Withdraw functionality', function () {
        it('withdraws tokens successfully', async function () {
            const withdrawAmount = hre.ethers.parseEther('1.0');
            
            console.log('Withdrawing', hre.ethers.formatEther(withdrawAmount), 'tokens');
            
            const tx = await werc20.withdraw(withdrawAmount, {
                gasLimit: GAS_LIMIT
            });
            const receipt = await tx.wait();
            
            console.log('Withdraw transaction hash:', receipt.hash);
            console.log('Gas used:', receipt.gasUsed.toString());
            
            // Check that transaction was successful
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
            
            // Look for Withdrawal event
            const withdrawEvent = receipt.logs.find(log => {
                try {
                    const parsed = werc20.interface.parseLog(log);
                    return parsed && parsed.name === 'Withdrawal';
                } catch {
                    return false;
                }
            });
            
            const parsed = werc20.interface.parseLog(withdrawEvent);
            console.log('Withdrawal event:', parsed.args);
            expect(parsed.args.src).to.equal(signer.address);
            expect(parsed.args.wad).to.equal(withdrawAmount);
            console.log('No Withdrawal event found (expected for mock implementation)');
        });

        it('withdraws different amounts', async function () {
            const amounts = [
                hre.ethers.parseEther('0.05'),  // Very small amount
                hre.ethers.parseEther('0.5'),   // Small amount
                hre.ethers.parseEther('3.0'),   // Medium amount
                hre.ethers.parseEther('7.5'),   // Large amount
            ];

            for (const amount of amounts) {
                console.log('Withdrawing', hre.ethers.formatEther(amount), 'tokens');
                
                const tx = await werc20.withdraw(amount, {
                    gasLimit: GAS_LIMIT
                });
                const receipt = await tx.wait();
                
                console.log('Gas used for', hre.ethers.formatEther(amount), 'withdrawal:', receipt.gasUsed.toString());
                
                expect(receipt.status).to.equal(1);
                expect(receipt.gasUsed).to.be.greaterThan(0);
            }
        });

        it('withdraws zero amount (edge case)', async function () {
            const zeroAmount = hre.ethers.parseEther('0');
            
            console.log('Withdrawing zero amount');
            
            const tx = await werc20.withdraw(zeroAmount, {
                gasLimit: GAS_LIMIT
            });
            const receipt = await tx.wait();
            
            console.log('Gas used for zero withdrawal:', receipt.gasUsed.toString());
            
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
        });
    });
});