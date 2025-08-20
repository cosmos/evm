const { expect } = require('chai');
const hre = require('hardhat');
const {
    DEFAULT_GAS_LIMIT,
    LARGE_GAS_LIMIT,
    LOW_GAS_LIMIT,
    PANIC_ASSERT_0x01,
    PANIC_DIVISION_BY_ZERO_0x12,
    PANIC_ARRAY_OUT_OF_BOUND_0x32
} = require('./common');
const {
    decodeRevertReason,
    analyzeFailedTransaction,
    verifyTransactionRevert,
    verifyOutOfGasError
} = require('./test_helper')


describe('Standard Revert Cases E2E Tests', function () {
    let standardRevertTestContract, simpleWrapper, signer;
    let analysis;

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

        analysis = null;
    });

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
            try {
                await standardRevertTestContract.arrayOutOfBounds();
                expect.fail('View call should have reverted');
            } catch (error) {
                decodedReason = decodeRevertReason(error.data)
            }
            expect(decodedReason).contains(PANIC_ARRAY_OUT_OF_BOUND_0x32)
        });

        it('should handle array out of bounds (Transaction Panic error)', async function () {
            try {
                const tx = await standardRevertTestContract.arrayOutOfBoundsTx({ gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyTransactionRevert(analysis, PANIC_ARRAY_OUT_OF_BOUND_0x32)
        });

        it('should capture revert reason through eth_getTransactionReceipt', async function () {
            try {
                const tx = await standardRevertTestContract.standardRevert("Test message", { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyTransactionRevert(analysis, "Test message")
        });
    });

    describe('Complex Revert Scenarios', function () {
        it('should handle multiple calls with revert', async function () {
            try {
                const tx = await standardRevertTestContract.multipleCallsWithRevert({ gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyTransactionRevert(analysis, "Multiple calls revert")
        });

        it('should handle try-catch revert scenario', async function () {
            try {
                const tx = await standardRevertTestContract.tryCatchRevert(true, { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyTransactionRevert(analysis, "Internal function revert")
        });

        it('should handle wrapper contract revert', async function () {
            const contractAddress = await standardRevertTestContract.getAddress();
            try {
                const tx = await simpleWrapper.wrappedStandardCall(contractAddress, "Wrapper test", { gasLimit: DEFAULT_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have reverted');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyTransactionRevert(analysis, "Wrapper test")
        });
    });

    describe('OutOfGas Error Cases', function () {
        it('should handle standard contract OutOfGas', async function () {
            try {
                const tx = await standardRevertTestContract.standardOutOfGas({ gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash);
            }
            verifyOutOfGasError(analysis)
        });

        it('should handle expensive computation OutOfGas', async function () {
            try {
                const tx = await standardRevertTestContract.expensiveComputation(10000, { gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash);
            }
            verifyOutOfGasError(analysis)
        });

        it('should handle expensive storage OutOfGas', async function () {
            try {
                const tx = await standardRevertTestContract.expensiveStorage(100, { gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash);
            }
            verifyOutOfGasError(analysis)
        });

        it('should handle wrapper OutOfGas', async function () {
            const contractAddress = await standardRevertTestContract.getAddress();
            try {
                const tx = await simpleWrapper.wrappedOutOfGasCall(contractAddress, { gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash);
            }
            verifyOutOfGasError(analysis)
        });

        it('should analyze OutOfGas error through transaction receipt', async function () {
            try {
                const tx = await standardRevertTestContract.standardOutOfGas({ gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash);
            }
            verifyOutOfGasError(analysis)
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
                    expectedReason: "Standard error"
                },
                {
                    name: 'Require Revert',
                    call: async () => {
                        const tx = await standardRevertTestContract.requireRevert(100, 50, { gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedReason: "Value exceeds threshold"
                },
                {
                    name: 'Assert Revert',
                    call: async () => {
                        const tx = await standardRevertTestContract.assertRevert({ gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedReason: PANIC_ASSERT_0x01
                },
                {
                    name: 'Division by Zero (Transaction)',
                    call: async () => {
                        const tx = await standardRevertTestContract.divisionByZeroTx({ gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedReason: PANIC_DIVISION_BY_ZERO_0x12
                },
                {
                    name: 'Array Out of Bounds (Transaction)',
                    call: async () => {
                        const tx = await standardRevertTestContract.arrayOutOfBoundsTx({ gasLimit: DEFAULT_GAS_LIMIT });
                        await tx.wait();
                    },
                    expectedReason: PANIC_ARRAY_OUT_OF_BOUND_0x32
                }
            ];

            // View functions that don't create receipts but still revert
            const viewTestCases = [
                {
                    name: 'Division by Zero (View)',
                    call: async () => await standardRevertTestContract.divisionByZero(),
                    expectedReason: PANIC_DIVISION_BY_ZERO_0x12
                },
                {
                    name: 'Array Out of Bounds (View)',
                    call: async () => await standardRevertTestContract.arrayOutOfBounds(),
                    expectedReason: PANIC_ARRAY_OUT_OF_BOUND_0x32
                }
            ];

            // Test transaction-based functions
            for (const testCase of transactionTestCases) {
                try {
                    await testCase.call();
                    expect.fail(`${testCase.name} should have reverted`);
                } catch (error) {
                    analysis = await analyzeFailedTransaction(error.receipt.hash);
                }
                verifyTransactionRevert(analysis, testCase.expectedReason)
            }
            
            // Test view functions (no receipts)
            for (const testCase of viewTestCases) {
                try {
                    await testCase.call();
                    expect.fail(`${testCase.name} should have reverted`);
                } catch (error) {
                    decodedReason = decodeRevertReason(error.data)
                }
                expect(decodedReason).contains(testCase.expectedReason)
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