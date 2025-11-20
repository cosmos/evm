const { expect } = require('chai')
const hre = require('hardhat')
const {
    STAKING_PRECOMPILE_ADDRESS,
    DEFAULT_GAS_LIMIT,
    parseValidator,
    findEvent, waitWithTimeout, RETRY_DELAY_FUNC
} = require('../common')

describe('StakingI – createValidator', function() {
    const GAS_LIMIT = DEFAULT_GAS_LIMIT // skip gas estimation for simplicity

    let staking, signer

    before(async () => {
        [signer] = await hre.ethers.getSigners()

        // Instantiate the StakingI precompile contract
        staking = await hre.ethers.getContractAt('StakingI', STAKING_PRECOMPILE_ADDRESS)
    })

    it('should create a validator successfully', async function() {
        // Define the validator’s descriptive metadata
        const description = {
            moniker: 'TestValidator',
            identity: 'id123',
            website: 'https://example.com',
            securityContact: 'sec@example.com',
            details: 'unit-test validator',
        }

        // Set initial commission parameters (18-decimal precision)
        const commissionRates = {
            rate: hre.ethers.parseUnits('0.05', 18), // 5%
            maxRate: hre.ethers.parseUnits('0.20', 18), // 20%
            maxChangeRate: hre.ethers.parseUnits('0.01', 18), // 1%
        }

        // Configure the remaining createValidator arguments
        const minSelfDelegation = 1
        const pubkey = 'nfJ0axJC9dhta1MAE1EBFaVdxxkYzxYrBaHuJVjG//M='
        const deposit = hre.ethers.parseEther('1') // self-delegate 1 native token

        // Submit the createValidator transaction
        const tx = await staking.connect(signer).createValidator(
            description,
            commissionRates,
            minSelfDelegation,
            signer.address,
            pubkey,
            deposit,
            { gasLimit: GAS_LIMIT }
        )

        // Wait for 2 confirmations and log the transaction hash
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)
        console.log('Transaction hash:', receipt.hash)

        // Find and parse the CreateValidator event from the transaction logs
        const parsed = findEvent(
            receipt.logs,
            staking.interface,
            'CreateValidator'
        )

        expect(parsed, 'CreateValidator event must be emitted').to.exist
        expect(parsed.args.validatorAddress).to.equal(signer.address)
        expect(parsed.args.value).to.equal(deposit)

        // Retrieve and log the on-chain Validator struct
        const rawInfo = await staking.validator(signer.address)
        console.log('Validator info:', rawInfo)

        // Parse the raw tuple into a structured object
        const info = parseValidator(rawInfo)

        // Verify that each field matches the expected values
        expect(info.operatorAddress.toLowerCase()).to.equal(signer.address.toLowerCase())
        expect(info.consensusPubkey).to.equal(pubkey)
        expect(info.jailed).to.be.false
        expect(info.status).to.equal(3n) // BondStatus.Bonded === 3
        expect(info.tokens).to.equal(deposit)
        expect(info.delegatorShares).to.be.gt(0n)
        expect(info.description.moniker).to.equal(description.moniker)
        expect(info.description.identity).to.equal(description.identity)
        expect(info.description.website).to.equal(description.website)
        expect(info.description.securityContact).to.equal(description.securityContact)
        expect(info.description.details).to.equal(description.details)
        expect(info.unbondingHeight).to.equal(0n)
        expect(info.unbondingTime).to.equal(0n)
        expect(info.commission).to.equal(commissionRates.rate)
        expect(info.minSelfDelegation).to.equal(BigInt(minSelfDelegation))

        // --- editValidator ---

        // prepare edit parameters: only update 'details'
        const editDescription = {
            moniker: '[do-not-modify]',
            identity: '[do-not-modify]',
            website: '[do-not-modify]',
            securityContact: '[do-not-modify]',
            details: 'updated unit-test validator',
        }
        const DO_NOT_MODIFY = -1

        // send editValidator tx
        const editTx = await staking.connect(signer).editValidator(
            editDescription,
            signer.address,
            DO_NOT_MODIFY,    // leave commissionRate unchanged
            DO_NOT_MODIFY,    // leave minSelfDelegation unchanged
            { gasLimit: GAS_LIMIT }
        )
        const editReceipt = await waitWithTimeout(editTx, 20000, RETRY_DELAY_FUNC)
        console.log('EditValidator tx hash:', editTx.hash)

        // parse EditValidator event
        const editEvt = findEvent(editReceipt.logs, staking.interface, 'EditValidator')
        expect(editEvt, 'EditValidator event must be emitted').to.exist
        expect(editEvt.args.validatorAddress).to.equal(signer.address)
        expect(editEvt.args.commissionRate).to.equal(DO_NOT_MODIFY)
        expect(editEvt.args.minSelfDelegation).to.equal(DO_NOT_MODIFY)

        // verify on-chain state after edit
        const updatedInfo = parseValidator(await staking.validator(signer.address))
        // only the "details" field is updated:
        expect(updatedInfo.description.details).to.equal(editDescription.details)
        // the other fields are unchanged:
        expect(updatedInfo.description.moniker).to.equal(description.moniker)
        expect(updatedInfo.description.identity).to.equal(description.identity)
        expect(updatedInfo.description.website).to.equal(description.website)
        expect(updatedInfo.description.securityContact).to.equal(description.securityContact)

        const pageReq = { key: '0x', offset: 0, limit: 100, countTotal: false, reverse: false }
        const out = await staking.validators('', pageReq)
        const validators = out.validators.map(parseValidator)
        expect(validators.length).to.be.gte(2)
        expect(validators[1].operatorAddress.toLowerCase()).to.equal(signer.address.toLowerCase())
    })
})
