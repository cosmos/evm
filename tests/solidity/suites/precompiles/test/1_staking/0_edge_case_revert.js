const { expect } = require('chai');
const hre = require('hardhat');
const {
    STAKING_PRECOMPILE_ADDRESS,
    LARGE_GAS_LIMIT,
    waitWithTimeout, RETRY_DELAY_FUNC
} = require('../common');

describe('Staking – edge case revert test', function () {
    const GAS_LIMIT = LARGE_GAS_LIMIT;

    let stakingReverter, staking, signer;
    let validatorAddress;

    before(async function () {
        [signer] = await hre.ethers.getSigners();
        
        // Get staking precompile interface
        staking = await hre.ethers.getContractAt('StakingI', STAKING_PRECOMPILE_ADDRESS);
        
        // Deploy StakingReverter contract with some native balance
        const StakingReverterFactory = await hre.ethers.getContractFactory('StakingReverter');
        stakingReverter = await StakingReverterFactory.deploy({
            value: hre.ethers.parseEther('1.0'), // Fund contract with 1 ETH
            gasLimit: GAS_LIMIT
        });
        await waitWithTimeout(stakingReverter.deploymentTransaction(), 20000, RETRY_DELAY_FUNC)

        validatorAddress = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql';
        
        console.log('StakingReverter deployed at:', await stakingReverter.getAddress());
        console.log('Using validator address:', validatorAddress);
    });

    describe('Edge case: callPrecompileBeforeAndAfterRevert with numTimes=1', function () {
        it('should execute exactly two delegate operations', async function () {
            const contractAddress = await stakingReverter.getAddress();
            
            // Get initial delegation before test
            let initialShares, initialBalance;
            [initialShares, initialBalance] = await staking.delegation(contractAddress, validatorAddress);
            console.log('Initial delegation shares:', initialShares.toString());
            console.log('Initial delegation balance:', initialBalance.amount.toString());

            // Call the edge case method with numTimes = 1
            console.log('Calling callPrecompileBeforeAndAfterRevert with numTimes=1');
            
            const tx = await stakingReverter.callPrecompileBeforeAndAfterRevert(1, validatorAddress, {
                gasLimit: GAS_LIMIT
            });
            await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC);
            const receipt = await tx.wait();
            
            console.log('Transaction hash:', receipt.hash);
            console.log('Gas used:', receipt.gasUsed.toString());
            
            // Verify transaction succeeded
            expect(receipt.status).to.equal(1);
            expect(receipt.gasUsed).to.be.greaterThan(0);
            
            // Check final delegation state
            const [finalShares, finalBalance] = await staking.delegation(contractAddress, validatorAddress);
            console.log('Final delegation shares:', finalShares.toString());
            console.log('Final delegation balance:', finalBalance.amount.toString());
            
            // Calculate expected final amount (initial + 2 delegate operations of 10 wei each)
            const expectedFinalAmount = BigInt(initialBalance.amount) + (2n * 10n);
            
            console.log('Expected final amount:', expectedFinalAmount.toString());
            console.log('Actual final amount:', finalBalance.amount.toString());
            
            // Verify exactly two delegate operations were executed
            // According to the pattern: one before the loop + one after the loop = 2 total
            expect(finalBalance.amount).to.equal(expectedFinalAmount);
            
            // Verify shares increased appropriately
            expect(finalShares).to.be.greaterThan(initialShares);
            
            console.log('✓ Edge case test passed: exactly two delegate operations executed');
        });
    });
});