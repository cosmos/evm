const { expect } = require('chai')
const hre = require('hardhat')
const { findEvent, waitWithTimeout, RETRY_DELAY_FUNC} = require('../common')

describe('ERC20 Precompile', function () {
    let erc20, owner, spender, recipient
    const GAS_LIMIT = 1_000_000 // skip gas estimation for simplicity

    before(async function () {
        [owner, spender, recipient] = await hre.ethers.getSigners()
        erc20 = await hre.ethers.getContractAt(
            'IERC20Metadata',
            '0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE'
        )
    })

    it('should return the name', async function () {
        const name = await erc20.name()
        expect(name).to.contain('Test Token')
    })

    it('should return the symbol', async function () {
        const symbol = await erc20.symbol()
        expect(symbol).to.contain('TEST')
    })

    it('should return the decimals', async function () {
        const decimals = await erc20.decimals()
        expect(decimals).to.equal(18)
    })

    it('should return the total supply', async function () {
        const totalSupply = await erc20.totalSupply()
        expect(totalSupply).to.be.gt(0n)
    })

    it('should return the balance of the owner', async function () {
        const balance = await erc20.balanceOf(owner.address)
        expect(balance).to.be.gt(0n)
    })

    it('should return zero allowance by default', async function () {
        const allowance = await erc20.allowance(owner.address, spender.address)
        expect(allowance).to.equal(0n)
    })

    it('should transfer tokens', async function () {
        const amount = hre.ethers.parseEther('1')
        const prev   = await erc20.balanceOf(spender.address)

        const tx = await erc20.connect(owner).transfer(spender.address, amount)
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)

        const transferEvent = findEvent(receipt.logs, erc20.interface, 'Transfer')
        expect(transferEvent, 'Transfer event must be emitted').to.exist
        expect(transferEvent.args.from).to.equal(owner.address)
        expect(transferEvent.args.to).to.equal(spender.address)
        expect(transferEvent.args.value).to.equal(amount)

        const after = await erc20.balanceOf(spender.address)
        expect(after - prev).to.equal(amount)
    })

    it('should transfer tokens using transferFrom', async function () {
        const amount = hre.ethers.parseEther('0.5')

        // owner gives spender permission to move amount
        const approvalTx = await erc20.
            connect(owner)
            .approve(spender.address, amount, {gasLimit: GAS_LIMIT})
        const approvalReceipt = await waitWithTimeout(approvalTx, 20000, RETRY_DELAY_FUNC)
        console.log(`Approval transaction hash: ${approvalTx.hash}`)

        const approvalEvent = findEvent(approvalReceipt.logs, erc20.interface, 'Approval')
        expect(approvalEvent, 'Approval event must be emitted').to.exist
        expect(approvalEvent.args.owner).to.equal(owner.address)
        expect(approvalEvent.args.spender).to.equal(spender.address)
        expect(approvalEvent.args.value).to.equal(amount)

        // record pre-transfer balances and allowance
        const prevBalance    = await erc20.balanceOf(recipient.address)
        const prevAllowance  = await erc20.allowance(owner.address, spender.address)
        console.log(`Pre-transfer balance of recipient: ${prevBalance}`)
        console.log(`Pre-transfer allowance of spender: ${prevAllowance}`)

        // spender pulls from owner → recipient
        const tx = await erc20
            .connect(spender)
            .transferFrom(owner.address, recipient.address, amount, {gasLimit: GAS_LIMIT})
        const receipt = await waitWithTimeout(tx, 20000, RETRY_DELAY_FUNC)
        console.log(`Transfer transaction hash: ${tx.hash}`)

        const transferEvent = findEvent(receipt.logs, erc20.interface, 'Transfer')
        expect(transferEvent, 'Transfer event must be emitted').to.exist
        expect(transferEvent.args.from).to.equal(owner.address)
        expect(transferEvent.args.to).to.equal(recipient.address)
        expect(transferEvent.args.value).to.equal(amount)

        // post-transfer checks
        const afterBalance   = await erc20.balanceOf(recipient.address)
        const afterAllowance = await erc20.allowance(owner.address, spender.address)

        // recipient should gain exactly `amount`
        expect(afterBalance - prevBalance).to.equal(amount)

        // allowance should have decreased by `amount`
        expect(afterAllowance).to.equal(prevAllowance - amount)
    })
})
