const { expect } = require('chai')
const hre = require('hardhat')
const {
    DISTRIBUTION_PRECOMPILE_ADDRESS,
    DEFAULT_GAS_LIMIT
} = require('../common')

describe('DistributionI – getParams', function () {
    const GAS_LIMIT = DEFAULT_GAS_LIMIT

    let distribution, signer

    before(async () => {
        [signer] = await hre.ethers.getSigners()
        distribution = await hre.ethers.getContractAt('DistributionI', DISTRIBUTION_PRECOMPILE_ADDRESS)
    })

    it('should retrieve distribution module parameters successfully', async function () {
        const params = await distribution.getParams({ gasLimit: GAS_LIMIT })
        
        // Verify parameter properties exist and have correct types
        expect(params.communityTax).to.exist
        expect(params.withdrawAddrEnabled).to.exist
        
        // The communityTax is a Dec struct which can return as [value, precision] array or object
        if (Array.isArray(params.communityTax)) {
            expect(params.communityTax).to.have.lengthOf(2)
            expect(params.communityTax[0]).to.be.a('bigint') // value
            expect(params.communityTax[1]).to.be.a('bigint') // precision  
        } else {
            expect(params.communityTax).to.be.an('object')
            expect(params.communityTax).to.have.property('value')
            expect(params.communityTax).to.have.property('precision')
        }
        
        expect(params.withdrawAddrEnabled).to.be.a('boolean')
        
        console.log('✓ All distribution parameters retrieved and validated')
        console.log('Parameter values:')
        if (Array.isArray(params.communityTax)) {
            console.log('  - Community Tax:', params.communityTax[0].toString(), 'with', params.communityTax[1].toString(), 'precision')
        } else {
            console.log('  - Community Tax:', params.communityTax.value ? params.communityTax.value.toString() : 'N/A', 
                       'with precision', params.communityTax.precision ? params.communityTax.precision.toString() : 'N/A')
        }
        console.log('  - Withdraw Address Enabled:', params.withdrawAddrEnabled)
    })
})