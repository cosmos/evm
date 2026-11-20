const { expect } = require('chai');
const hre = require('hardhat');
const { LARGE_GAS_LIMIT, LOW_GAS_LIMIT } = require('./common');
const {
    analyzeFailedTransaction,
    parseEthersError,
    verifyOutOfGasError
} = require('./test_helper')

describe('Precompile Revert Cases E2E Tests', function () {
    let revertTestContract, precompileWrapper;
    let validValidatorAddress, invalidValidatorAddress;
    let analysis, decodedReason;
    let stakingIface, distributionIface;

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
        invalidValidatorAddress = 'cosmosvaloper10jmp6sgh4cc6zt3e8gw05wavvejgr5pinvalid';
        
        console.log('RevertTestContract deployed at:', await revertTestContract.getAddress());
        console.log('PrecompileWrapper deployed at:', await precompileWrapper.getAddress());

        // Load per-precompile ABIs (with module-specific custom errors).
        // These are interfaces (abstract), so use getContractAt to access their interface/ABI.
        stakingIface = (await hre.ethers.getContractAt('StakingI', hre.ethers.ZeroAddress)).interface;
        distributionIface = (await hre.ethers.getContractAt('DistributionI', hre.ethers.ZeroAddress)).interface;

        analysis = null;
        decodedReason = null;
    });

    describe('Direct Precompile Call Reverts', function () {
        it('should handle direct staking precompile revert', async function () {
            try {
                await revertTestContract.directStakingRevert.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                expect.fail('Call should have reverted');
            } catch (e) {
                const parsed = parseEthersError(stakingIface, e.data);
                expect(parsed.name).to.equal("InvalidAddress");
                expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
            }
        });

        it('should handle direct distribution precompile revert', async function () {            
            try {
                await revertTestContract.directDistributionRevert.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                expect.fail('Call should have reverted');
            } catch (e) {
                const parsed = parseEthersError(distributionIface, e.data);
                expect(parsed.name).to.equal("InvalidAddress");
                expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
            }
        });

        it('should handle direct bank precompile revert', async function () {
            // directBankRevert is a view function, so it should revert immediately
            try {
                await revertTestContract.directBankRevert.staticCall();
                expect.fail('Call should have reverted');
            } catch (error) {
                const parsed = parseEthersError(null, error.data);
                expect(parsed.name).to.equal("Error");
                expect(String(parsed.args[0])).to.include("intended revert");
            }
        });

        it('should capture precompile revert reason through transaction receipt', async function () {
            try {
                await revertTestContract.directStakingRevert.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                expect.fail('Call should have reverted');
            } catch (e) {
                const parsed = parseEthersError(stakingIface, e.data);
                expect(parsed.name).to.equal("InvalidAddress");
                expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
            }
        });
    });

    describe('Precompile Call Via Contract Reverts', function () {
        it('should handle precompile call via contract revert', async function () {            
            try {
                await revertTestContract.precompileViaContractRevert.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                expect.fail('Call should have reverted');
            } catch (e) {
                const parsed = parseEthersError(stakingIface, e.data);
                expect(parsed.name).to.equal("InvalidAddress");
                expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
            }
        });

        it('should handle multiple precompile calls with revert', async function () {            
            try {
                await revertTestContract.multiplePrecompileCallsWithRevert.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                expect.fail('Call should have reverted');
            } catch (e) {
                const parsed = parseEthersError(stakingIface, e.data);
                expect(parsed.name).to.equal("InvalidAddress");
                expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
            }
        });

        it('should handle wrapper contract precompile revert', async function () {
            try {
                await precompileWrapper.wrappedStakingCall.staticCall(invalidValidatorAddress, 1, { gasLimit: LARGE_GAS_LIMIT });
                expect.fail('Call should have reverted');
            } catch (e) {
                const parsed = parseEthersError(stakingIface, e.data);
                expect(parsed.name).to.equal("InvalidAddress");
                expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
            }
        });

        it('should capture wrapper revert reason via transaction receipt', async function () {
            try {
                await precompileWrapper.wrappedDistributionCall.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT });
                expect.fail('Call should have reverted');
            } catch (e) {
                const parsed = parseEthersError(distributionIface, e.data);
                expect(parsed.name).to.equal("InvalidAddress");
                expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
            }
        });
    });

    describe('Precompile OutOfGas Error Cases', function () {
        it('should handle direct precompile OutOfGas', async function () {
            // Use a very low gas limit to trigger OutOfGas on precompile calls            
            try {
                const tx = await revertTestContract.directStakingOutOfGas(validValidatorAddress, { gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyOutOfGasError(analysis)
        });

        it('should handle precompile via contract OutOfGas', async function () {            
            try {
                const tx = await revertTestContract.precompileViaContractOutOfGas(validValidatorAddress, { gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyOutOfGasError(analysis)
        });

        it('should handle wrapper precompile OutOfGas', async function () {
            try {
                const tx = await precompileWrapper.wrappedOutOfGasCall(validValidatorAddress, { gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyOutOfGasError(analysis)
        });

        it('should analyze precompile OutOfGas error through transaction receipt', async function () {
            try {
                const tx = await revertTestContract.directStakingOutOfGas(validValidatorAddress, { gasLimit: LOW_GAS_LIMIT });
                await tx.wait();
                expect.fail('Transaction should have failed with OutOfGas');
            } catch (error) {
                analysis = await analyzeFailedTransaction(error.receipt.hash)
            }
            verifyOutOfGasError(analysis)
        });
    });

    describe('Comprehensive Precompile Error Analysis', function () {
        it('should properly decode various precompile error types from transaction receipts', async function () {
            const testCases = [
                {
                    name: 'Staking Precompile Revert',
                    call: () => revertTestContract.directStakingRevert.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT }),
                    expectedReason: {
                        name: "InvalidAddress",
                        args: [
                            invalidValidatorAddress,
                        ],
                    },
                    iface: stakingIface,

                },
                {
                    name: 'Distribution Precompile Revert',
                    call: () => revertTestContract.directDistributionRevert.staticCall(invalidValidatorAddress, { gasLimit: LARGE_GAS_LIMIT }),
                    expectedReason: "InvalidAddress",
                    expectedReason: {
                        name: "InvalidAddress",
                        args: [
                            invalidValidatorAddress,
                        ],
                    },
                    iface: distributionIface,
                }
            ];

            for (const testCase of testCases) {
                try {
                    await testCase.call();
                    expect.fail(`${testCase.name} should have reverted`);
                } catch (e) {
                    const parsed = parseEthersError(testCase.iface, e.data);
                    expect(parsed.name).to.equal(testCase.expectedReason.name);

                    const expArgs = testCase.expectedReason.args || [];
                    for (let i = 0; i < expArgs.length; i++) {
                        expect(String(parsed.args[i])).to.include(String(expArgs[i]));
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
                    } catch (e) {
                        const parsed = parseEthersError(distributionIface, e.data);
                        expect(parsed.name).to.equal("InvalidAddress");
                        expect(String(parsed.args[0])).to.include(invalidValidatorAddress);
                    }
                }
            }
        });
    });
});