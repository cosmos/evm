const { expect } = require('chai');
const hre = require('hardhat');

describe('Standard Revert Cases E2E Tests', function () {
    let standardRevertTestContract, simpleWrapper, signer;
    
    // Gas limits for testing
    const DEFAULT_GAS_LIMIT = 1000000;
    const LARGE_GAS_LIMIT = 10000000;

    before(async function () {
        [signer] = await hre.ethers.getSigners();
        
        // Deploy StandardRevertTestContract
        const StandardRevertTestContractFactory = await hre.ethers.getContractFactory('StandardRevertTestContract');
        standardRevertTestContract = await StandardRevertTestContractFactory.deploy({
            value: hre.ethers.parseEther('1.0'), // Fund with 1 ETH
            gasLimit: LARGE_GAS_LIMIT
        });
        await standardRevertTestContract.waitForDeployment();
        
        // Deploy SimpleWrapper
        const SimpleWrapperFactory = await hre.ethers.getContractFactory('SimpleWrapper');
        simpleWrapper = await SimpleWrapperFactory.deploy({
            value: hre.ethers.parseEther('1.0'), // Fund with 1 ETH
            gasLimit: LARGE_GAS_LIMIT
        });
        await simpleWrapper.waitForDeployment();
        
        // Verify successful deployment
        const contractAddress = await standardRevertTestContract.getAddress();
        const wrapperAddress = await simpleWrapper.getAddress();
        console.log('StandardRevertTestContract deployed at:', contractAddress);
        console.log('SimpleWrapper deployed at:', wrapperAddress);
    });

    /**
     * Helper function to check if an error is an out of gas error using transaction analysis
     */
    async function isOutOfGasError(error) {
        if (!error.receipt) {
            return false;
        }

        const analysis = await analyzeFailedTransaction(error.receipt.hash);
        
        // Check if the error message from the analysis contains "out of gas"
        return analysis.errorMessage && analysis.errorMessage.toLowerCase().includes('out of gas');
    }

    /**
     * Helper function to decode hex error data from transaction receipt
     */
    function decodeRevertReason(errorData) {
        if (!errorData || errorData === '0x') {
            return null; // No error data (common for OutOfGas)
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
            expect.fail(`Failed to decode revert reason: ${error.message}`);
        }
    }

    /**
     * Helper function to analyze transaction receipt for revert information
     */
    async function analyzeFailedTransaction(txHash) {
        const receipt = await hre.ethers.provider.getTransactionReceipt(txHash);
        const tx = await hre.ethers.provider.getTransaction(txHash);

        expect(receipt.status).to.equal(0, 'Transaction should have failed');

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
            // For OutOfGas errors, error.data might be undefined
            let decodedReason = null;
            if (error.data) {
                decodedReason = decodeRevertReason(error.data);
                console.log(`  Revert Reason: ${decodedReason}`);
            } else {
                console.log('  Revert Reason: No error data available');
            }

            return {
                status: receipt.status,
                gasUsed: receipt.gasUsed,
                gasLimit: tx.gasLimit,
                errorData: error.data,
                decodedReason: decodedReason,
                errorMessage: error.message
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

    describe('Standard Contract Call Reverts', function () {
        it('should handle standard revert with custom message', async function () {
            const customMessage = "Custom revert message";
            
            // Verify that the transaction reverts
            let transactionReverted = false;
            
            try {
                const tx = await standardRevertTestContract.standardRevert(customMessage, { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;

            // Verify we can capture the revert reason via static call
            try {
                await standardRevertTestContract.standardRevert.staticCall(customMessage);
                expect.fail('Static call should have reverted');
            } catch (staticError) {
                expect(staticError.message).to.include(customMessage);
                // Error message validated above
            }
        });

        it('should handle require revert with proper error message', async function () {
            const value = 100;
            const threshold = 50;
            
            let transactionReverted = false;
            
            try {
                const tx = await standardRevertTestContract.requireRevert(value, threshold, { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
            
            // Verify we can capture the revert reason via static call
            try {
                await standardRevertTestContract.requireRevert.staticCall(value, threshold);
                expect.fail('Static call should have reverted');
            } catch (staticError) {
                expect(staticError.message).to.include("Value exceeds threshold");
                // Error message validated above
            }
            
            // Verify successful case (no revert when value < threshold)
            const successTx = await standardRevertTestContract.requireRevert(25, 50, { gasLimit: DEFAULT_GAS_LIMIT });
            const receipt = await successTx.wait();
            expect(receipt.status).to.equal(1, 'Transaction should succeed when value < threshold');
        });

        it('should handle assert revert (Panic error)', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await standardRevertTestContract.assertRevert({ gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
            
            // Verify we can capture the revert reason via static call
            try {
                await standardRevertTestContract.assertRevert.staticCall();
                expect.fail('Static call should have reverted');
            } catch (staticError) {
                // Check for either "panic" or "assert(false)" as different nodes may return different messages
                const hasExpectedError = staticError.message.includes("panic") || staticError.message.includes("assert(false)");
                expect(hasExpectedError).to.be.true;
                // Error message validated above
            }
        });

        it('should handle division by zero (View Panic error)', async function () {
            let transactionReverted = false;
            
            try {
                await standardRevertTestContract.divisionByZero();
                expect.fail('View call should have reverted');
            } catch (error) {
                transactionReverted = true;
                expect(error.message).to.include("division or modulo by zero");
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should handle division by zero (Transaction Panic error)', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await standardRevertTestContract.divisionByZeroTx({ gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
                expect(error.receipt).to.exist;
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should handle array out of bounds (View Panic error)', async function () {
            let transactionReverted = false;
            
            try {
                await standardRevertTestContract.arrayOutOfBounds();
                expect.fail('View call should have reverted');
            } catch (error) {
                transactionReverted = true;
                expect(error.message).to.include("out-of-bounds");
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should handle array out of bounds (Transaction Panic error)', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await standardRevertTestContract.arrayOutOfBoundsTx({ gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
                expect(error.receipt).to.exist;
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should capture revert reason through eth_getTransactionReceipt', async function () {
            try {
                const tx = await standardRevertTestContract.standardRevert("Test message", { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                // Must have receipt for failed transaction
                expect(error.receipt).to.exist;
                const analysis = await analyzeFailedTransaction(error.receipt.hash);
                expect(analysis.status).to.equal(0);
                expect(analysis.decodedReason).to.include("Test message");
            }
        });
    });

    describe('Complex Revert Scenarios', function () {
        it('should handle multiple calls with revert', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await standardRevertTestContract.multipleCallsWithRevert({ gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should handle try-catch revert scenario', async function () {
            let transactionReverted = false;
            
            try {
                const tx = await standardRevertTestContract.tryCatchRevert(true, { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
        });

        it('should handle wrapper contract revert', async function () {
            const contractAddress = await standardRevertTestContract.getAddress();
            
            let transactionReverted = false;
            
            try {
                const tx = await simpleWrapper.wrappedStandardCall(contractAddress, "Wrapper test", { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                transactionReverted = true;
            }
            
            expect(transactionReverted).to.be.true;
        });
    });

    describe('OutOfGas Error Cases', function () {
        it('should handle standard contract OutOfGas', async function () {
            // Use a very low gas limit to trigger OutOfGas
            const lowGasLimit = 50000;

            let transactionFailed = false;
            let isOutOfGas = false;
            
            try {
                const tx = await standardRevertTestContract.standardOutOfGas({ gasLimit: lowGasLimit });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                transactionFailed = true;
                isOutOfGas = await isOutOfGasError(error);
                
                // Analyze the failed transaction
                expect(error.receipt).to.exist;
                const analysis = await analyzeFailedTransaction(error.receipt.hash);
                expect(analysis.gasUsed.toString()).to.equal(lowGasLimit.toString());
            }
            
            expect(transactionFailed).to.be.true;
            expect(isOutOfGas).to.be.true;
        });

        it('should handle expensive computation OutOfGas', async function () {
            const lowGasLimit = 100000;

            let transactionFailed = false;
            let isOutOfGas = false;
            
            try {
                const tx = await standardRevertTestContract.expensiveComputation(10000, { gasLimit: lowGasLimit });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                transactionFailed = true;
                isOutOfGas = await isOutOfGasError(error);
            }
            
            expect(transactionFailed).to.be.true;
            expect(isOutOfGas).to.be.true;
        });

        it('should handle expensive storage OutOfGas', async function () {
            const lowGasLimit = 200000;
            let transactionFailed = false;
            let isOutOfGas = false;
            
            try {
                const tx = await standardRevertTestContract.expensiveStorage(100, { gasLimit: lowGasLimit });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                transactionFailed = true;
                isOutOfGas = await isOutOfGasError(error);
            }
            
            expect(transactionFailed).to.be.true;
            expect(isOutOfGas).to.be.true;
        });

        it('should handle wrapper OutOfGas', async function () {
            const contractAddress = await standardRevertTestContract.getAddress();

            let transactionFailed = false;
            let isOutOfGas = false;
            
            try {
                const tx = await simpleWrapper.wrappedOutOfGasCall(contractAddress, { gasLimit: 100000 });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                transactionFailed = true;
                isOutOfGas = await isOutOfGasError(error);
            }
            
            expect(transactionFailed).to.be.true;
            expect(isOutOfGas).to.be.true;
        });

        it('should analyze OutOfGas error through transaction receipt', async function () {
            const testGasLimit = 50000;
            
            let isOutOfGas = false;
            let analysis = null;
            
            try {
                const tx = await standardRevertTestContract.standardOutOfGas({ gasLimit: testGasLimit });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                isOutOfGas = await isOutOfGasError(error);
                
                // Must have receipt for failed transaction
                expect(error.receipt).to.exist;
                analysis = await analyzeFailedTransaction(error.receipt.hash);
                expect(analysis.status).to.equal(0);
            }
            
            expect(isOutOfGas).to.be.true;
            expect(analysis).to.not.be.null;
        });
    });

    describe('Comprehensive Error Analysis', function () {
        it('should properly decode various error types from transaction receipts', async function () {
            // Transaction-based functions that create receipts
            const transactionTestCases = [
                {
                    name: 'Standard Revert',
                    call: async () => {
                        const tx = await standardRevertTestContract.standardRevert("Standard error", { gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedInReason: "Standard error"
                },
                {
                    name: 'Require Revert',
                    call: async () => {
                        const tx = await standardRevertTestContract.requireRevert(100, 50, { gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedInReason: "Value exceeds threshold"
                },
                {
                    name: 'Assert Revert',
                    call: async () => {
                        const tx = await standardRevertTestContract.assertRevert({ gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedInReason: "Panic(1)"
                },
                {
                    name: 'Division by Zero (Transaction)',
                    call: async () => {
                        const tx = await standardRevertTestContract.divisionByZeroTx({ gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedInReason: "Panic(18)"
                },
                {
                    name: 'Array Out of Bounds (Transaction)',
                    call: async () => {
                        const tx = await standardRevertTestContract.arrayOutOfBoundsTx({ gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedInReason: "Panic(50)"
                }
            ];

            // View functions that don't create receipts but still revert
            const viewTestCases = [
                {
                    name: 'Division by Zero (View)',
                    call: async () => await standardRevertTestContract.divisionByZero(),
                    expectedInError: "division or modulo by zero"
                },
                {
                    name: 'Array Out of Bounds (View)',
                    call: async () => await standardRevertTestContract.arrayOutOfBounds(),
                    expectedInError: "out-of-bounds"
                }
            ];

            // Test transaction-based functions
            for (const testCase of transactionTestCases) {
                try {
                    await testCase.call();
                    expect.fail(`${testCase.name} should have reverted`);
                } catch (error) {
                    // Must have receipt for all failed transactions
                    expect(error.receipt).to.exist;
                    const analysis = await analyzeFailedTransaction(error.receipt.hash);
                    expect(analysis.status).to.equal(0);
                    if (testCase.expectedInReason) {
                        expect(analysis.decodedReason).to.include(testCase.expectedInReason);
                    }
                }
            }
            
            // Test view functions (no receipts)
            for (const testCase of viewTestCases) {
                try {
                    await testCase.call();
                    expect.fail(`${testCase.name} should have reverted`);
                } catch (error) {
                    // View functions don't have receipts
                    expect(error.receipt).to.be.undefined;
                    // Check error message directly
                    expect(error.message).to.include(testCase.expectedInError);
                }
            }
        });

        it('should verify error data is properly hex-encoded in receipts', async function () {
            try {
                const tx = await standardRevertTestContract.standardRevert("Hex encoding test", { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                // Must have receipt for failed transaction
                expect(error.receipt).to.exist;
                
                // Simulate the call to get error data
                let errorCaught = false;
                try {
                    const contractAddress = await standardRevertTestContract.getAddress();
                    await hre.ethers.provider.call({
                        to: contractAddress,
                        data: standardRevertTestContract.interface.encodeFunctionData('standardRevert', ['Hex encoding test']),
                        gasLimit: DEFAULT_GAS_LIMIT
                    });
                    expect.fail('Call should have reverted');
                } catch (callError) {
                    errorCaught = true;
                    expect(callError.data).to.exist;
                    expect(callError.data).to.match(/^0x/, 'Error data must be hex-encoded');
                    
                    const decoded = decodeRevertReason(callError.data);
                    expect(decoded).to.include("Hex encoding test");
                }
                expect(errorCaught).to.equal(true, 'Call must revert with error');
            }
        });
    });
});