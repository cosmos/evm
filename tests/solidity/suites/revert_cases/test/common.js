// Common constants and helper utilities for precompile tests
const { expect } = require('chai');

const STAKING_PRECOMPILE_ADDRESS = '0x0000000000000000000000000000000000000800'
const BECH32_PRECOMPILE_ADDRESS = '0x0000000000000000000000000000000000000400'
const DISTRIBUTION_PRECOMPILE_ADDRESS = '0x0000000000000000000000000000000000000801'
const BANK_PRECOMPILE_ADDRESS = '0x0000000000000000000000000000000000000804'
const GOV_PRECOMPILE_ADDRESS = '0x0000000000000000000000000000000000000805'
const SLASHING_PRECOMPILE_ADDRESS = '0x0000000000000000000000000000000000000806'
const P256_PRECOMPILE_ADDRESS = '0x0000000000000000000000000000000000000100'
const WERC20_ADDRESS = '0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE'

// Default gas limits used across tests
const DEFAULT_GAS_LIMIT = 1_000_000
const LARGE_GAS_LIMIT = 10_000_000

// Helper to convert the raw tuple returned by staking.validator() into an object
function parseValidator (raw) {
    return {
        operatorAddress: raw[0],
        consensusPubkey: raw[1],
        jailed: raw[2],
        status: raw[3],
        tokens: raw[4],
        delegatorShares: raw[5],
        description: raw[6],
        unbondingHeight: raw[7],
        unbondingTime: raw[8],
        commission: raw[9],
        minSelfDelegation: raw[10]
    }
}

// Utility to parse logs and return the first matching event by name
function findEvent (logs, iface, eventName) {
    for (const log of logs) {
        try {
            const parsed = iface.parseLog(log)
            if (parsed && parsed.name === eventName) {
                return parsed
            }
        } catch {
            // ignore logs that do not match the contract interface
        }
    }
    return null
}

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
            const offsetHex = reasonHex.slice(0, 64); // Get offset (should be 0x20 = 32)
            const offset = parseInt(offsetHex, 16);
            
            if (offset === 32) { // Standard ABI encoding has offset of 32
                const reasonLength = parseInt(reasonHex.slice(64, 128), 16); // Get string length from next 32 bytes
                const reasonBytes = reasonHex.slice(128, 128 + reasonLength * 2); // Get string data
                return Buffer.from(reasonBytes, 'hex').toString('utf8');
            } else {
                // Fallback for non-standard encoding
                const reasonLength = parseInt(reasonHex.slice(0, 64), 16); // Get string length
                const reasonBytes = reasonHex.slice(128, 128 + reasonLength * 2); // Get string data
                return Buffer.from(reasonBytes, 'hex').toString('utf8');
            }
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
 * Helper function to verify decoded revert reason
 */
function verifyTransactionRevert(analysis, expectedRevertReason) {
    expect(analysis).to.not.be.null
    expect(analysis.status).to.equal(0); // Failed transaction
    expect(analysis.errorData).to.not.be.null;
    expect(analysis.decodedReason).contains(expectedRevertReason, "unexpected revert reason")
}

/**
 * Helper function to verify out of gas error
 */
function verifyOutOfGasError(analysis) {
    expect(analysis).to.not.be.null
    expect(analysis.status).to.equal(0); // Failed transaction
    expect(analysis.gasUsed).to.be.equal(analysis.gasLimit)
}

module.exports = {
    STAKING_PRECOMPILE_ADDRESS,
    BECH32_PRECOMPILE_ADDRESS,
    DISTRIBUTION_PRECOMPILE_ADDRESS,
    BANK_PRECOMPILE_ADDRESS,
    GOV_PRECOMPILE_ADDRESS,
    SLASHING_PRECOMPILE_ADDRESS,
    P256_PRECOMPILE_ADDRESS,
    WERC20_ADDRESS,
    DEFAULT_GAS_LIMIT,
    LARGE_GAS_LIMIT,
    parseValidator,
    findEvent,
    decodeRevertReason,
    analyzeFailedTransaction,
    verifyTransactionRevert,
    verifyOutOfGasError
}