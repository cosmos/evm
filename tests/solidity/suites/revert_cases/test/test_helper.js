const { expect } = require('chai');
const hre = require('hardhat');

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
 * Parse revert data into a structured error object when possible.
 *
 * - Standard revert string: Error(string)
 * - Panic(uint256)
 * - Common precompile custom errors (e.g. InvalidAddress, QueryFailed, MsgServerFailed)
 *
 * Returns:
 *   { name, args, signature, raw } or null
 */
function parseEthersError(iface, errorData) {
    if (!errorData || errorData === '0x') {
        throw new Error('missing revert data (e.data is empty)');
    }

    const raw = errorData;
    const cleanHex = raw.startsWith('0x') ? raw : `0x${raw}`;

    // Standard revert formats handled by our string decoder.
    const cleanNo0x = cleanHex.slice(2);
    if (cleanNo0x.startsWith('08c379a0')) {
        const reason = decodeRevertReason(cleanHex);
        return { name: 'Error', args: [reason], signature: 'Error(string)', raw: cleanHex };
    }
    if (cleanNo0x.startsWith('4e487b71')) {
        // Panic(uint256) — decode the numeric code for direct comparison.
        // Layout: selector(4) + uint256 code (32 bytes).
        const codeHex = `0x${cleanNo0x.slice(8, 8 + 64)}`;
        const code = Number(BigInt(codeHex));
        return { name: 'Panic', args: [code], signature: 'Panic(uint256)', raw: cleanHex };
    }

    try {
        if (!iface) {
            throw new Error('no interface provided for custom error decoding');
        }
        const parsed = iface.parseError(cleanHex);
        if (!parsed) {
            throw new Error(`unable to decode error data: ${cleanHex}`);
        }
        return parsed;
    } catch {
        throw new Error(`unable to decode error data: ${cleanHex}`);
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
            decodedReason: decodeRevertReason(error.data),
            errorMessage: error.message
        };
    }
    
    return {
        status: receipt.status,
        gasUsed: receipt.gasUsed,
        gasLimit: tx.gasLimit,
        errorData: null,
        decodedReason: null,
        errorMessage: null
    };
}

/**
 * Helper function to verify out of gas error
 */
function verifyOutOfGasError(analysis) {
    expect(analysis).to.not.be.null;
    expect(analysis.status).to.equal(0); // Failed transaction
    expect(analysis.gasUsed).to.be.equal(analysis.gasLimit);
    expect(analysis.errorMessage.toLowerCase()).include('out of gas');
}

module.exports = {
    parseValidator,
    findEvent,
    parseEthersError,
    analyzeFailedTransaction,
    verifyOutOfGasError
}