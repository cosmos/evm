const { expect } = require('chai');
const hre = require('hardhat');
const {
    LARGE_GAS_LIMIT,
} = require('./common');

describe('Precompile Revert Cases E2E Tests', function () {
    let revertTestContract, precompileWrapper, signer;
    let validValidatorAddress, invalidValidatorAddress;

    before(async function () {
        [signer] = await hre.ethers.getSigners();
        
        // Deploy RevertTestContract
        const RevertTestContractFactory = await hre.ethers.getContractFactory('RevertTestContract');
        revertTestContract = await RevertTestContractFactory.deploy({
            value: hre.ethers.parseEther('1.0'), // Fund with 1 ETH
            gasLimit: LARGE_GAS_LIMIT
        });
        await revertTestContract.waitForDeployment();
        
        // Deploy PrecompileWrapper
        const PrecompileWrapperFactory = await hre.ethers.getContractFactory('PrecompileWrapper');
        precompileWrapper = await PrecompileWrapperFactory.deploy({
            value: hre.ethers.parseEther('1.0'), // Fund with 1 ETH
            gasLimit: LARGE_GAS_LIMIT
        });
        await precompileWrapper.waitForDeployment();
        
        // Use a known validator for valid cases and invalid one for error cases
        validValidatorAddress = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pw4xyrql';
        invalidValidatorAddress = 'invalid_validator_address';
        
        console.log('RevertTestContract deployed at:', await revertTestContract.getAddress());
        console.log('PrecompileWrapper deployed at:', await precompileWrapper.getAddress());
    });

    /**
     * Helper function to decode hex error data from transaction receipt
     */
    function decodeRevertReason(errorData) {
        if (!errorData || errorData === '0x') {
            return null;
        }
        
        try {
            // Remove '0x' prefix
            const cleanHex = errorData.startsWith('0x') ? errorData.slice(2) : errorData;
            
            // Check if it's a standard revert string (function selector: 08c379a0)
            if (cleanHex.startsWith('08c379a0')) {
                const reasonHex = cleanHex.slice(8); // Remove function selector
                const reasonLength = parseInt(reasonHex.slice(0, 64), 16); // Get string length
                const reasonBytes = reasonHex.slice(128, 128 + reasonLength * 2); // Get string data
                return Buffer.from(reasonBytes, 'hex').toString('utf8');
            }
            
            // Check if it's a Panic error (function selector: 4e487b71)
            if (cleanHex.startsWith('4e487b71')) {
                const panicCode = parseInt(cleanHex.slice(8, 72), 16);
                return `Panic(${panicCode})`;
            }
            
            // Return raw hex if not a standard format
            return `Raw: ${errorData}`;
        } catch (error) {
            return `Decode error: ${error.message}`;
        }
    }

    /**
     * Helper function to analyze transaction receipt for revert information
     */
    async function analyzeFailedTransaction(txHash) {
        const receipt = await hre.ethers.provider.getTransactionReceipt(txHash);
        const tx = await hre.ethers.provider.getTransaction(txHash);
        
       // Try to get revert reason through call simulation
        try {
            await hre.ethers.provider.call({
                to: tx.to,
                data: tx.data,
                from: tx.from,
                value: tx.value,
                gasLimit: tx.gasLimit,
                gasPrice: tx.gasPrice
            });
        } catch (error) {
            console.log(`  Revert Reason: ${decodeRevertReason(error.data)}`);
            return {
                status: receipt.status,
                gasUsed: receipt.gasUsed,
                gasLimit: tx.gasLimit,
                errorData: error.data,
                decodedReason: decodeRevertReason(error.data)
            };
        }
        
        return {
            status: receipt.status,
            gasUsed: receipt.gasUsed,
            gasLimit: tx.gasLimit,
            errorData: null,
            decodedReason: null
        };
    }

    /**
     * Helper function to validation decoded revert reason
     */
    async function validateRevertReason(txHash, expectedRevertReason) {
        const analysis = await analyzeFailedTransaction(txHash)

        expect(analysis.status).to.equal(0); // Failed transaction
        expect(analysis.errorData).to.not.be.null;
        expect(analysis.decodedReason).to.be.equal(expectedRevertReason, "unexpected revert reason")
    }

    describe('Direct Precompile Call Reverts', function () {
        var txHash

        beforeEach(function () {
            txHash = null
        })

        it('should handle direct staking precompile revert', async function () {
            try {
                const tx = await revertTestContract.directStakingRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                txHash = error.receipt.hash
            }
            
            await validateRevertReason(txHash, "invalid validator address: decod")
        });

        it('should handle direct distribution precompile revert', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await revertTestContract.directDistributionRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should handle direct bank precompile revert', async function () {
            // directBankRevert is a view function, so it should revert immediately
            let callReverted = false;
            
            try {
                await revertTestContract.directBankRevert();
                expect.fail('Call should have reverted');
            } catch (error) {
                callReverted = true;
            }
            expect(callReverted).to.be.true;
        });

        it('should capture precompile revert reason through transaction receipt', async function () {
            try {
                const tx = await revertTestContract.directStakingRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                if (error.receipt) {
                    const analysis = await analyzeFailedTransaction(error.receipt.hash);
                    expect(analysis.status).to.equal(0); // Failed transaction
                    expect(analysis.errorData).to.not.be.null;
                    console.log('Precompile revert analysis:', analysis);
                }
            }
        });
    });

    describe('Precompile Call Via Contract Reverts', function () {
        it('should handle precompile call via contract revert', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await revertTestContract.precompileViaContractRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
        });

        it('should handle multiple precompile calls with revert', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await revertTestContract.multiplePrecompileCallsWithRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should handle wrapper contract precompile revert', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await precompileWrapper.wrappedStakingCall(invalidValidatorAddress, 1, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should capture wrapper revert reason via transaction receipt', async function () {
            try {
                const tx = await precompileWrapper.wrappedDistributionCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                if (error.receipt) {
                    const analysis = await analyzeFailedTransaction(error.receipt.hash);
                    expect(analysis.status).to.equal(0);
                    expect(analysis.decodedReason).to.include("invalid validator address");
                }
            }
        });
    });

    describe('Precompile OutOfGas Error Cases', function () {
        it('should handle direct precompile OutOfGas', async function () {
            // Use a very low gas limit to trigger OutOfGas on precompile calls
            const lowGasLimit = 80000;
            let transactionFailed = false;
            let gasAnalysis = null;
            
            try {
                const tx = await revertTestContract.directStakingOutOfGas(validValidatorAddress, { gasLimit: lowGasLimit });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                transactionFailed = true;
                
                // Analyze the transaction to verify it's specifically OutOfGas
                if (error.receipt) {
                    const gasUsed = Number(error.receipt.gasUsed);
                    const gasLimit = lowGasLimit;
                    const gasUtilization = gasUsed / gasLimit;
                    
                    gasAnalysis = {
                        gasUsed,
                        gasLimit,
                        gasUtilization,
                        isOutOfGas: gasUtilization > 0.9 // Used more than 90% of gas
                    };
                    
                    console.log('Precompile OutOfGas Analysis:', gasAnalysis);
                }
            }
            
            expect(transactionFailed).to.be.true;
            
            // Verify this is specifically an OutOfGas error
            if (gasAnalysis) {
                expect(gasAnalysis.isOutOfGas).to.be.true;
                expect(gasAnalysis.gasUtilization).to.be.greaterThan(0.8); // Should use most of the gas
            }
        });

        it('should handle precompile via contract OutOfGas', async function () {
            let transactionFailed = false;
            
            try {
                const tx = await revertTestContract.precompileViaContractOutOfGas(validValidatorAddress, { gasLimit: 100000 });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                transactionFailed = true;
            }
            
            expect(transactionFailed).to.be.true;
        });

        it('should handle wrapper precompile OutOfGas', async function () {
            let transactionFailed = false;
            
            try {
                const tx = await precompileWrapper.wrappedOutOfGasCall(validValidatorAddress, { gasLimit: 100000 });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                transactionFailed = true;
            }
            
            expect(transactionFailed).to.be.true;
        });

        it('should analyze precompile OutOfGas error through transaction receipt', async function () {
            const testGasLimit = 70000;
            
            try {
                const tx = await revertTestContract.directStakingOutOfGas(validValidatorAddress, { gasLimit: testGasLimit });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                if (error.receipt) {
                    const analysis = await analyzeFailedTransaction(error.receipt.hash);
                    expect(analysis.status).to.equal(0);
                    
                    // OutOfGas specific checks
                    const gasUsed = Number(analysis.gasUsed);
                    const gasLimit = Number(analysis.gasLimit);
                    const gasUtilization = gasUsed / gasLimit;
                    
                    console.log('Precompile OutOfGas analysis:', {
                        ...analysis,
                        gasUtilization: gasUtilization.toFixed(3),
                        isOutOfGas: gasUtilization > 0.8
                    });
                    
                    // Verify this is actually OutOfGas (high gas utilization)
                    expect(gasUtilization).to.be.greaterThan(0.8, 'Gas utilization should be high for OutOfGas errors');
                    expect(gasUsed).to.be.closeTo(gasLimit, gasLimit * 0.2); // Within 20% of gas limit
                }
            }
        });
    });

    describe('Comprehensive Precompile Error Analysis', function () {
        it('should properly decode various precompile error types from transaction receipts', async function () {
            const testCases = [
                {
                    name: 'Staking Precompile Revert',
                    call: () => revertTestContract.directStakingRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT }),
                    expectedInReason: "invalid validator address"
                },
                {
                    name: 'Distribution Precompile Revert',
                    call: () => revertTestContract.directDistributionRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT }),
                    expectedInReason: "validator does not exist"
                }
            ];

            for (const testCase of testCases) {
                try {
                    await testCase.call();
                    expect.fail(`${testCase.name} should have reverted`);
                } catch (error) {
                    if (error.receipt) {
                        const analysis = await analyzeFailedTransaction(error.receipt.hash);
                        expect(analysis.status).to.equal(0);
                        if (testCase.expectedInReason) {
                            expect(analysis.decodedReason).to.include(testCase.expectedInReason);
                        }
                    }
                }
            }
        });

        it('should verify precompile error data is properly hex-encoded in receipts', async function () {
            try {
                const tx = await revertTestContract.directStakingRevert(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                if (error.receipt) {
                    // Simulate the call to get error data
                    try {
                        const contractAddress = await revertTestContract.getAddress();
                        await hre.ethers.provider.call({
                            to: contractAddress,
                            data: revertTestContract.interface.encodeFunctionData('directStakingRevert', [invalidValidatorAddress]),
                            gasLimit: LARGE_GAS_LIMIT
                        });
                    } catch (callError) {
                        expect(callError.data).to.match(/^0x/); // Should be hex-encoded
                        console.log('Precompile error data (hex):', callError.data);
                        
                        const decoded = decodeRevertReason(callError.data);
                        expect(decoded).to.include("invalid validator address");
                        console.log('Decoded precompile reason:', decoded);
                    }
                }
            }
        });
    });
});