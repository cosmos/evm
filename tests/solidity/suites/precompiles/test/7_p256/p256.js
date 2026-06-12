const { expect } = require('chai')
const hre = require('hardhat')

describe('P256 Precompile', function () {
    const P256_ADDRESS = '0x0000000000000000000000000000000000000100'
    const GAS_LIMIT = 1_000_000

    let signer

    before(async () => {
        [signer] = await hre.ethers.getSigners()
    })

    it('verifies valid P256 signature', async function () {
        // Valid P256 signature data for "hello world" message
        // Hash: b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
        // R: 59bf72e33c396c3f60ed34b03e407f8ac4285fcb458ad8595bf6513ec1767695
        // S: e9df97c1facdd47acffb3a82523f6384e43522c26a6ce3f3c183ea3d71e899e7
        // X: 9e66f04a4bf0a41c979fa022720881b336dfebdc74cf84614ca349262633e3e5
        // Y: 4655cfa9f2a8472cb5d577d241bae970424df4213d0e71ff5abac4409c01da6f
        
        const hash = '0xb94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9'
        const r = '0x59bf72e33c396c3f60ed34b03e407f8ac4285fcb458ad8595bf6513ec1767695'
        const s = '0xe9df97c1facdd47acffb3a82523f6384e43522c26a6ce3f3c183ea3d71e899e7'
        const x = '0x9e66f04a4bf0a41c979fa022720881b336dfebdc74cf84614ca349262633e3e5'
        const y = '0x4655cfa9f2a8472cb5d577d241bae970424df4213d0e71ff5abac4409c01da6f'

        const input = hash + r.slice(2) + s.slice(2) + x.slice(2) + y.slice(2)

        console.log('Input length:', input.length / 2 - 1, 'bytes')
        expect(input.length).to.equal(322) // 160 bytes * 2 + 2 for 0x

        // Make direct call to P256 precompile
        const result = await hre.ethers.provider.call({
            to: P256_ADDRESS,
            data: input,
            gasLimit: GAS_LIMIT
        })

        console.log('P256 precompile result:', result)
        
        // Valid signature should return 32 bytes with value 1
        expect(result).to.equal('0x0000000000000000000000000000000000000000000000000000000000000001')
        
        console.log('✓ P256 signature verification successful')
    })
})