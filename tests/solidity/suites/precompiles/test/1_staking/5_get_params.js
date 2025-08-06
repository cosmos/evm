const { expect } = require('chai')
const hre = require('hardhat')
const {
    STAKING_PRECOMPILE_ADDRESS,
    DEFAULT_GAS_LIMIT
} = require('../common')

describe('StakingI – getParams', function () {
    const GAS_LIMIT = DEFAULT_GAS_LIMIT

    let staking, signer

    before(async () => {
        [signer] = await hre.ethers.getSigners()
        staking = await hre.ethers.getContractAt('StakingI', STAKING_PRECOMPILE_ADDRESS)
    })

    it('should retrieve staking module parameters successfully', async function () {
        console.log('staking.getParams')
        console.log(staking.getParams)
        const params = await staking.getParams({ gasLimit: GAS_LIMIT })

        // Verify parameter types and basic validity
        expect(params.unbondingTime).to.be.a('bigint')
        expect(params.unbondingTime).to.be.gt(0n) // Should be positive
        
        // These are uint32 so they come back as bigint in ethers v6
        expect(params.maxValidators).to.be.a('bigint')
        expect(params.maxValidators).to.be.gt(0n) // Should be positive
        
        expect(params.maxEntries).to.be.a('bigint')
        expect(params.maxEntries).to.be.gt(0n) // Should be positive
        
        expect(params.historicalEntries).to.be.a('bigint')
        expect(params.historicalEntries).to.be.gte(0n) // Can be 0 or positive
        
        expect(params.bondDenom).to.be.a('string')
        expect(params.bondDenom).to.not.be.empty // Should not be empty
        
        // minCommissionRate is a Dec struct which returns as [value, precision] array or object
        if (Array.isArray(params.minCommissionRate)) {
            expect(params.minCommissionRate).to.have.lengthOf(2)
            expect(params.minCommissionRate[0]).to.be.a('bigint') // value
            expect(params.minCommissionRate[1]).to.be.a('bigint') // precision
        } else {
            expect(params.minCommissionRate).to.be.an('object')
            expect(params.minCommissionRate).to.have.property('value')
            expect(params.minCommissionRate).to.have.property('precision')
        }
        
        console.log('✓ All staking parameters retrieved and validated')
        console.log('Parameter values:')
        console.log('  - Unbonding Time:', params.unbondingTime.toString(), 'nanoseconds')
        console.log('  - Max Validators:', params.maxValidators.toString())
        console.log('  - Max Entries:', params.maxEntries.toString())
        console.log('  - Historical Entries:', params.historicalEntries.toString())
        console.log('  - Bond Denomination:', params.bondDenom)
        if (Array.isArray(params.minCommissionRate)) {
            console.log('  - Min Commission Rate:', params.minCommissionRate[0].toString(), 'with', params.minCommissionRate[1].toString(), 'precision')
        } else {
            console.log('  - Min Commission Rate:', params.minCommissionRate.value ? params.minCommissionRate.value.toString() : 'N/A', 
                       'with precision', params.minCommissionRate.precision ? params.minCommissionRate.precision.toString() : 'N/A')
        }
    })
})
