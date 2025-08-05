const { expect } = require('chai')
const hre = require('hardhat')
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common')

describe('Gov Precompile', function () {
    const GOV_ADDRESS = '0x0000000000000000000000000000000000000805'
    const GAS_LIMIT = 1_000_000
    const COSMOS_ADDR = 'cosmos1cml96vmptgw99syqrrz8az79xer2pcgp95srxm'
    const GOV_MODULE_ADDR = 'cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn'

    let gov, signer, globalProposalId

    before(async () => {
        [signer] = await hre.ethers.getSigners()
        gov = await hre.ethers.getContractAt('IGov', GOV_ADDRESS)
        
        // Create a single proposal to be reused across tests
        const jsonProposal = buildProposal(COSMOS_ADDR)
        const deposit = { denom: 'atest', amount: hre.ethers.parseEther('1') }

        const tx = await gov
            .connect(signer)
            .submitProposal(signer.address, jsonProposal, [deposit], { gasLimit: GAS_LIMIT })
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)

        const evt = findEvent(receipt.logs, gov.interface, 'SubmitProposal')
        
        if (!evt) {
            throw new Error('SubmitProposal event not found in receipt')
        }
        
        globalProposalId = evt.args.proposalId
        console.log('Global proposal ID created:', globalProposalId.toString())
    })

    // helper to craft a minimal bank send proposal in proto-json format
    function buildProposal(toCosmos) {
        const msg = {
            '@type': '/cosmos.bank.v1beta1.MsgSend',
            from_address: GOV_MODULE_ADDR,
            to_address: toCosmos,
            amount: [{ denom: 'atest', amount: '1' }],
        }

        const prop = {
            messages: [msg],
            metadata: 'ipfs://CID',
            title: 'test prop',
            summary: 'test prop',
            expedited: false,
        }

        return Buffer.from(JSON.stringify(prop))
    }

    it('queries the global proposal', async function () {
        const proposal = await gov.getProposal(globalProposalId)
        expect(proposal.id).to.equal(globalProposalId)
        expect(proposal.proposer).to.equal(signer.address)
        expect(proposal.title).to.equal('test prop')
        expect(proposal.summary).to.equal('test prop')
        expect(proposal.metadata).to.equal('ipfs://CID')
    })

    it('deposits on the global proposal', async function () {
        const amt = hre.ethers.parseEther('0.5')
        const deposit = { denom: 'atest', amount: amt }

        // Check balances before deposit
        const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address)

        const depTx = await gov
            .connect(signer)
            .deposit(signer.address, globalProposalId, [deposit], { gasLimit: GAS_LIMIT })
        const depRcpt = await waitWithTimeout(depTx, 20000, RETRY_DELAY_FUNC)

        // Check balances after deposit
        const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address)
        const gasFee = depRcpt.gasUsed * depRcpt.gasPrice

        // Verify balance changes (only gas fees should be deducted for gov deposit)
        expect(signerBalanceAfter).to.equal(signerBalanceBefore - amt - gasFee)

        const depEvt = findEvent(depRcpt.logs, gov.interface, 'Deposit')
        expect(depEvt, 'Deposit event must be emitted').to.exist
        expect(depEvt.args.proposalId).to.equal(globalProposalId)
        expect(depEvt.args.depositor).to.equal(signer.address)
    })

    it('votes on the global proposal', async function () {
        const voteTx = await gov
            .connect(signer)
            .vote(signer.address, globalProposalId, 1, 'simple vote', { gasLimit: GAS_LIMIT })
        const voteRcpt = await waitWithTimeout(voteTx, 20000, RETRY_DELAY_FUNC)
        const voteEvt = findEvent(voteRcpt.logs, gov.interface, 'Vote')
        expect(voteEvt, 'Vote event must be emitted').to.exist
        expect(voteEvt.args.option).to.equal(1)
        expect(voteEvt.args.proposalId).to.equal(globalProposalId)
        expect(voteEvt.args.voter).to.equal(signer.address)
    })

    it('votes with weighted options on the global proposal', async function () {
        const weightedOptions = [
            { option: 1, weight: '0.6' }, // Yes: 60%
            { option: 2, weight: '0.4' }  // Abstain: 40%
        ]

        const tx = await gov
            .connect(signer)
            .voteWeighted(signer.address, globalProposalId, weightedOptions, 'weighted vote', { gasLimit: GAS_LIMIT })
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)

        const evt = findEvent(receipt.logs, gov.interface, 'VoteWeighted')
        expect(evt, 'VoteWeighted event must be emitted').to.exist
        expect(evt.args.voter).to.equal(signer.address)
        expect(evt.args.proposalId).to.equal(globalProposalId)
        expect(evt.args.options.length).to.equal(2)
        expect(evt.args.options[0].option).to.equal(1)
        expect(evt.args.options[0].weight).to.equal('0.6')
        expect(evt.args.options[1].option).to.equal(2)
        expect(evt.args.options[1].weight).to.equal('0.4')
    })

    it('queries params and constitution', async function () {
        const params = await gov.getParams()
        expect(params.votingPeriod).to.be.a('bigint')
        expect(params.minDeposit.length).to.be.greaterThan(0)

        const constitution = await gov.getConstitution()
        expect(constitution).to.be.a('string')
    })

    it('queries votes for the global proposal', async function () {
        const pagination = { key: new Uint8Array(), offset: 0, limit: 10, countTotal: true, reverse: false }
        const votesResult = await gov.getVotes(globalProposalId, pagination)
        
        expect(votesResult.votes.length).to.be.greaterThan(0)
        expect(votesResult.votes[0].proposalId).to.equal(globalProposalId)
        expect(votesResult.votes[0].voter).to.equal(signer.address)
        expect(votesResult.pageResponse.total).to.be.greaterThan(0)
    })

    it('queries specific vote for the global proposal', async function () {
        const vote = await gov.getVote(globalProposalId, signer.address)
        expect(vote.proposalId).to.equal(globalProposalId)
        expect(vote.voter).to.equal(signer.address)
        expect(vote.options.length).to.be.greaterThan(0)
        expect(vote.metadata).to.be.a('string')
    })

    it('queries specific deposit for the global proposal', async function () {
        const depositResult = await gov.getDeposit(globalProposalId, signer.address)
        expect(depositResult.proposalId).to.equal(globalProposalId)
        expect(depositResult.depositor).to.equal(signer.address)
        expect(depositResult.amount.length).to.be.greaterThan(0)
        expect(depositResult.amount[0].denom).to.equal('atest')
    })

    it('queries all deposits for the global proposal', async function () {
        const pagination = { key: new Uint8Array(), offset: 0, limit: 10, countTotal: true, reverse: false }
        const depositsResult = await gov.getDeposits(globalProposalId, pagination)
        
        expect(depositsResult.deposits.length).to.be.greaterThan(0)
        expect(depositsResult.deposits[0].proposalId).to.equal(globalProposalId)
        expect(depositsResult.deposits[0].depositor).to.equal(signer.address)
        expect(depositsResult.deposits[0].amount.length).to.be.greaterThan(0)
        expect(depositsResult.pageResponse.total).to.be.greaterThan(0)
    })

    it('queries tally result for the global proposal', async function () {
        const tallyResult = await gov.getTallyResult(globalProposalId)
        expect(tallyResult.yes).to.be.a('string')
        expect(tallyResult.abstain).to.be.a('string')
        expect(tallyResult.no).to.be.a('string')
        expect(tallyResult.noWithVeto).to.be.a('string')
    })

    it('queries all proposals', async function () {
        const pagination = { key: new Uint8Array(), offset: 0, limit: 10, countTotal: true, reverse: false }
        const result = await gov.getProposals(0, signer.address, signer.address, pagination)
        
        expect(result.proposals.length).to.be.greaterThan(0)
        expect(result.pageResponse.total).to.be.greaterThan(0)
        
        const proposal = result.proposals.find(p => p.id === globalProposalId)
        expect(proposal).to.exist
        expect(proposal.proposer).to.equal(signer.address)
        expect(proposal.title).to.equal('test prop')
        expect(proposal.summary).to.equal('test prop')
    })

    // TODO: Add multiple depositors case.
    it('cancels a proposal', async function () {
        const proposalIdToCancel = globalProposalId
        
        // Calculate total deposits made (1 ETH initial + 0.5 ETH additional)
        const initialDeposit = hre.ethers.parseEther('1')
        const additionalDeposit = hre.ethers.parseEther('0.5')
        const totalDeposits = initialDeposit + additionalDeposit
        const expectedRefund = totalDeposits / 2n // 50% refund
        
        // Check balances before cancel
        const signerBalanceBefore = await hre.ethers.provider.getBalance(signer.address)
        
        const cancelTx = await gov
            .connect(signer)
            .cancelProposal(signer.address, proposalIdToCancel, {gasLimit: GAS_LIMIT})
        const cancelRcpt = await waitWithTimeout(cancelTx, 20000, RETRY_DELAY_FUNC)

        // Check balances after cancel
        const signerBalanceAfter = await hre.ethers.provider.getBalance(signer.address)
        const gasFee = cancelRcpt.gasUsed * cancelRcpt.gasPrice
        
        // Verify balance changes (50% refund minus gas fees)
        expect(signerBalanceAfter).to.equal(signerBalanceBefore + expectedRefund - gasFee)
        
        const cancelEvt = findEvent(cancelRcpt.logs, gov.interface, 'CancelProposal')

        expect(cancelEvt, 'CancelProposal event must be emitted').to.exist
        expect(cancelEvt.args.proposer).to.equal(signer.address)
        expect(cancelEvt.args.proposalId).to.equal(proposalIdToCancel)

        await expect(gov.getProposal(proposalIdToCancel)).to.be.reverted;
    })
})
