import asyncio
import time

import pytest
import web3
from eth_account import Account
from eth_bloom import BloomFilter
from eth_contract.erc20 import ERC20
from eth_contract.utils import broadcast_transaction
from eth_contract.utils import send_transaction as send_transaction_async
from eth_utils import big_endian_to_int
from hexbytes import HexBytes
from pystarport.utils import w3_wait_for_new_blocks_async

from .utils import (
    ACCOUNTS,
    ADDRS,
    DEFAULT_DENOM,
    KEYS,
    WEI_PER_DENOM,
    AsyncGreeter,
    AsyncTestRevert,
    Contract,
    RevertTestContract,
    address_to_bytes32,
    assert_transfer,
    bech32_to_eth,
    build_batch_tx,
    build_contract,
    contract_address,
    create_periodic_vesting_acct,
    do_multisig,
    recover_community,
    send_transaction,
    transfer_via_cosmos,
)


def test_simple(evm, check_reserve=True):
    """
    check number of validators
    """
    cli = evm.cosmos_cli()
    assert len(cli.validators()) > 0
    if check_reserve:
        # check vesting account
        cli = evm.cosmos_cli()
        denom = cli.get_params("evm")["evm_denom"]
        addr = cli.address("reserve")
        account = cli.account(addr)["account"]
        assert account["type"] == "/cosmos.vesting.v1beta1.DelayedVestingAccount"
        assert account["value"]["base_vesting_account"]["original_vesting"] == [
            {"denom": denom, "amount": "100000000000000000000"}
        ]


def test_vesting(evm, tmp_path):
    cli = evm.cosmos_cli()
    start_time = int(time.time())
    end_time = start_time + 3000
    name = f"vesting{start_time}"
    addr = cli.create_account(name)["address"]
    coin = f"1{DEFAULT_DENOM}"
    community = cli.address("community")
    rsp = cli.create_periodic_vesting_acct(addr, coin, end_time, from_=community)
    assert rsp["code"] == 0, rsp["raw_log"]
    create_periodic_vesting_acct(cli, tmp_path, coin, from_=community)


def test_transfer(evm):
    """
    check simple transfer tx success
    """
    cli = evm.cosmos_cli()
    addr_a = cli.address("community")
    addr_b = cli.address("reserve")
    assert_transfer(cli, addr_a, addr_b)


async def test_send_transaction(evm, check_gas=True):
    tx = {"to": ADDRS["signer1"], "value": 1000}
    receipt = await send_transaction_async(evm.async_w3, ACCOUNTS["community"], **tx)
    if check_gas:
        assert receipt.gasUsed == 21000


def test_events(evm, exp_gas_used=806200):
    w3 = evm.w3
    sender = ADDRS["community"]
    receiver = ADDRS["signer1"]
    contract = Contract("TestERC20A")
    contract.deploy(w3, exp_gas_used=exp_gas_used)
    erc20 = contract.contract
    amt = 10
    tx = erc20.functions.transfer(receiver, amt).build_transaction({"from": sender})
    txreceipt = send_transaction(w3, tx)
    assert len(txreceipt.logs) == 1
    expect_log = {
        "address": erc20.address,
        "topics": [
            ERC20.events.Transfer.topic,
            address_to_bytes32(sender),
            address_to_bytes32(receiver),
        ],
        "data": HexBytes(amt.to_bytes(32, "big")),
        "transactionIndex": 0,
        "logIndex": 0,
        "removed": False,
    }
    assert expect_log.items() <= txreceipt.logs[0].items()

    # check block bloom
    bloom = BloomFilter(
        big_endian_to_int(w3.eth.get_block(txreceipt.blockNumber).logsBloom)
    )
    assert HexBytes(erc20.address) in bloom
    for topic in expect_log["topics"]:
        assert topic in bloom

    block_logs = w3.eth.get_block_receipts(txreceipt.blockNumber)[0].logs[0]
    call = w3.provider.make_request
    tx_logs = call("eth_getTransactionLogs", [txreceipt.transactionHash])["result"][0]
    for k in expect_log:
        assert expect_log[k] == block_logs[k]
        if k == "address":
            assert expect_log[k] == w3.to_checksum_address(tx_logs[k])
        elif k == "data":
            assert expect_log[k].hex() == block_logs[k].hex() == tx_logs[k][2:]
        elif k == "topics":
            assert expect_log[k] == [HexBytes(t) for t in tx_logs[k]]
        elif k in ("transactionIndex", "logIndex"):
            assert expect_log[k] == int(tx_logs[k], 16)
        else:
            assert expect_log[k] == tx_logs[k]


@pytest.mark.asyncio
async def test_minimal_gas_price(evm):
    w3 = evm.async_w3
    tx = {
        "to": "0x0000000000000000000000000000000000000000",
        "value": 10000,
        "gasPrice": 1,
    }
    with pytest.raises(web3.exceptions.Web3RPCError, match="insufficient fee"):
        await send_transaction_async(w3, ACCOUNTS["community"], **tx)
    tx["gasPrice"] = await w3.eth.gas_price
    receipt = await send_transaction_async(w3, ACCOUNTS["signer1"], **tx)
    assert receipt.status == 1


@pytest.mark.asyncio
async def test_transaction(evm):
    w3 = evm.async_w3
    gas_price = await w3.eth.gas_price
    gas = 21000
    acct = ACCOUNTS["community"]
    sender = acct.address
    receiver = ADDRS["signer1"]

    data = {"to": ADDRS["community"], "value": 10000, "gasPrice": gas_price, "gas": gas}
    res = await send_transaction_async(w3, acct, **data)
    assert res["transactionIndex"] == 0

    with pytest.raises(web3.exceptions.Web3RPCError, match="tx already in mempool"):
        data["nonce"] = await w3.eth.get_transaction_count(sender) - 1
        await send_transaction_async(w3, acct, **data)

    data["nonce"] = await w3.eth.get_transaction_count(sender) + 1
    txhash = await broadcast_transaction(w3, acct, **data)

    data["nonce"] = await w3.eth.get_transaction_count(sender)
    receipt = await send_transaction_async(w3, acct, **data)
    assert receipt["status"] == 1

    receipt = await w3.eth.wait_for_transaction_receipt(txhash)
    assert receipt["status"] == 1

    with pytest.raises(web3.exceptions.Web3RPCError, match="intrinsic gas too low"):
        await send_transaction_async(
            w3,
            acct,
            to=receiver,
            value=10000,
            gasPrice=gas_price,
            gas=1,
        )

    with pytest.raises(web3.exceptions.Web3RPCError, match="insufficient fee"):
        await send_transaction_async(
            w3,
            acct,
            to=receiver,
            value=10000,
            gas=gas,
            gasPrice=1,
        )

    contracts = {
        "test_revert_1": AsyncTestRevert(KEYS["validator"]),
        "test_revert_2": AsyncTestRevert(KEYS["community"]),
        "greeter_1": AsyncGreeter(KEYS["signer1"]),
        "greeter_2": AsyncGreeter(KEYS["signer2"]),
    }
    await w3_wait_for_new_blocks_async(w3, 1)

    deployment_tasks = [contract.deploy(w3) for contract in contracts.values()]
    await asyncio.gather(*deployment_tasks)
    await w3_wait_for_new_blocks_async(w3, 1)

    call_tasks = []
    call_tasks.append(contracts["test_revert_1"].transfer(5 * (10**18) - 1))
    call_tasks.append(contracts["test_revert_2"].transfer(5 * (10**18)))
    call_tasks.append(contracts["greeter_1"].set_greeting("hello"))
    call_tasks.append(contracts["greeter_2"].set_greeting("world"))
    results = await asyncio.gather(*call_tasks, return_exceptions=True)

    # revert transaction for 1st, normal transaction for others
    statuses = [0, 1, 1, 1]
    valid_receipts = []

    for i, result in enumerate(results):
        if isinstance(result, Exception):
            if i != 0:
                raise result
        else:
            assert result["status"] == statuses[i]
            if result["status"] == 1:
                valid_receipts.append(result)

    await assert_receipt_transaction_and_block(w3, valid_receipts)


async def assert_receipt_transaction_and_block(w3, receipts):
    assert len(receipts) >= 1, "should have at least 1 valid receipt"
    block_number = await w3.eth.get_block_number()
    for receipt in receipts:
        assert receipt["blockNumber"] == block_number
    tx_indexes = [receipt["transactionIndex"] for receipt in receipts]
    assert len(tx_indexes) == len(set(tx_indexes)), "duplicate index found"
    block = await w3.eth.get_block(block_number)

    for receipt in receipts:
        tx_index = receipt["transactionIndex"]
        tx = await w3.eth.get_transaction_by_block(block_number, tx_index)
        assert tx["blockNumber"] == block_number
        assert tx["transactionIndex"] == receipt["transactionIndex"]
        assert tx["hash"] == receipt["transactionHash"]
        assert tx["hash"] in block["transactions"]
        assert tx["blockNumber"] == block["number"]


def test_exception(evm):
    w3 = evm.w3
    key = KEYS["community"]
    revert = RevertTestContract("TestRevert", private_key=key)
    revert.deploy(w3)
    contract = revert.contract
    with pytest.raises(web3.exceptions.ContractLogicError):
        send_transaction(
            w3,
            contract.functions.transfer(5 * (10**18) - 1).build_transaction(),
            key=key,
        )
    assert 0 == contract.caller.query()

    receipt = send_transaction(
        w3, contract.functions.transfer(5 * (10**18)).build_transaction(), key=key
    )
    assert receipt.status == 1, "should be successfully"
    assert 5 * (10**18) == contract.caller.query()


def test_message_call(evm, diff=5):
    "stress test the evm by doing message calls as much as possible"
    w3 = evm.w3
    key = KEYS["community"]
    msg = Contract("TestMessageCall", private_key=key)
    msg.deploy(w3)
    iterations = 13000
    addr = ADDRS["community"]
    tx = msg.contract.functions.test(iterations).build_transaction(
        {
            "from": addr,
            "nonce": w3.eth.get_transaction_count(addr),
        }
    )

    begin = time.time()
    tx["gas"] = w3.eth.estimate_gas(tx)
    elapsed = time.time() - begin
    print("elapsed:", elapsed)
    assert elapsed < diff  # should finish in reasonable time

    receipt = send_transaction(w3, tx, key=key)
    assert 22768266 == receipt.cumulativeGasUsed
    assert receipt.status == 1, "shouldn't fail"
    assert len(receipt.logs) == iterations


def test_log0(evm):
    """
    test compliance of empty topics behavior
    """
    w3 = evm.w3
    key = KEYS["community"]
    empty = Contract("TestERC20A", private_key=key)
    empty.deploy(w3)
    contract = empty.contract
    tx = contract.functions.test_log0().build_transaction({"from": ADDRS["community"]})
    receipt = send_transaction(w3, tx, key=key)
    assert len(receipt.logs) == 1
    log = receipt.logs[0]
    assert log.topics == []
    data = "0x68656c6c6f20776f726c64000000000000000000000000000000000000000000"
    assert log.data == HexBytes(data)


async def test_contract(evm, tmp_path):
    "test Greeter contract"
    cli = evm.cosmos_cli()
    recover_community(cli, tmp_path)
    w3 = evm.async_w3
    greeter = AsyncGreeter()
    await greeter.deploy(w3)
    assert "Hello" == await greeter.greet()
    # change
    receipt = await greeter.set_greeting("world")
    assert "world" == await greeter.greet()
    assert receipt.status == 1


def test_batch_tx(evm):
    "send multiple eth txs in single cosmos tx should be disabled"
    w3 = evm.w3
    cli = evm.cosmos_cli()
    sender = ADDRS["community"]
    recipient = ADDRS["signer1"]
    nonce = w3.eth.get_transaction_count(sender)
    res = build_contract("TestERC20A")
    contract = w3.eth.contract(abi=res["abi"], bytecode=res["bytecode"])
    deploy_tx = contract.constructor().build_transaction(
        {"from": sender, "nonce": nonce}
    )
    contract = w3.eth.contract(address=contract_address(sender, nonce), abi=res["abi"])
    transfer_tx1 = contract.functions.transfer(recipient, 1000).build_transaction(
        {"from": sender, "nonce": nonce + 1, "gas": 200000}
    )
    transfer_tx2 = contract.functions.transfer(recipient, 1000).build_transaction(
        {"from": sender, "nonce": nonce + 2, "gas": 200000}
    )

    cosmos_tx, tx_hashes = build_batch_tx(
        w3, cli, [deploy_tx, transfer_tx1, transfer_tx2], key=KEYS["community"]
    )
    rsp = cli.broadcast_tx_json(cosmos_tx)
    assert rsp["code"] == 18
    assert f"got {len(tx_hashes)}" in rsp["raw_log"]


def test_refund_unused_gas_when_contract_tx_reverted(evm):
    """
    Call a smart contract method that reverts with very high gas limit

    Call tx receipt should be status 0 (fail)
    Fee is gasUsed * effectiveGasPrice
    """
    w3 = evm.w3
    key = KEYS["community"]
    sender = ADDRS["community"]
    revert = RevertTestContract("TestRevert", private_key=key)
    revert.deploy(w3)
    contract = revert.contract
    more_than_enough_gas = 1000000

    balance_bef = w3.eth.get_balance(sender)
    receipt = send_transaction(
        w3,
        contract.functions.transfer(5 * (10**18) - 1).build_transaction(
            {"gas": more_than_enough_gas}
        ),
        key=key,
    )
    balance_aft = w3.eth.get_balance(sender)

    assert receipt["status"] == 0, "should be a failed tx"
    assert receipt["gasUsed"] != more_than_enough_gas
    assert (
        balance_bef - balance_aft == receipt["gasUsed"] * receipt["effectiveGasPrice"]
    )


@pytest.mark.skip(reason="skipping batch tx test")
def test_failed_transfer_tx(evm):
    """
    It's possible to include a failed transfer transaction in batch tx
    """
    w3 = evm.w3
    cli = evm.cosmos_cli()
    sender = ADDRS["community"]
    recipient = ADDRS["signer1"]
    nonce = w3.eth.get_transaction_count(sender)
    half_balance = w3.eth.get_balance(sender) // 3 + 1

    # build batch tx, the third tx will fail, but will be included in block
    # because of the batch tx.
    transfer1 = {"from": sender, "nonce": nonce, "to": recipient, "value": half_balance}
    transfer2 = {
        "from": sender,
        "nonce": nonce + 1,
        "to": recipient,
        "value": half_balance,
    }
    transfer3 = {
        "from": sender,
        "nonce": nonce + 2,
        "to": recipient,
        "value": half_balance,
    }
    cosmos_tx, tx_hashes = build_batch_tx(
        w3, cli, [transfer1, transfer2, transfer3], KEYS["community"]
    )
    rsp = cli.broadcast_tx_json(cosmos_tx)
    assert rsp["code"] == 0, rsp["raw_log"]

    receipts = [w3.eth.wait_for_transaction_receipt(h) for h in tx_hashes]
    assert receipts[0].status == receipts[1].status == 1
    assert receipts[2].status == 0

    # check traceTransaction
    rsps = [
        w3.provider.make_request("debug_traceTransaction", [h.hex()]) for h in tx_hashes
    ]
    for rsp, receipt in zip(rsps, receipts):
        if receipt.status == 1:
            result = rsp["result"]
            assert not result["failed"]
            assert receipt.gasUsed == result["gas"]
        else:
            assert rsp["result"] == {
                "failed": True,
                "gas": 0,
                # "gas": 21000, TODO: mmsqe
                "returnValue": "0x",
                "structLogs": [],
            }


def test_multisig(evm, tmp_path):
    cli = evm.cosmos_cli()
    do_multisig(cli, tmp_path, "signer1", "signer2", "multitest1")


def test_multisig_cosmos(evm, tmp_path):
    cli = evm.cosmos_cli()
    recover1 = "recover1"
    recover2 = "recover2"
    amt = 6_000_000_000_000_000_000 // WEI_PER_DENOM
    addr_recover1 = cli.create_account(
        recover1,
        coin_type=118,
        key_type="secp256k1",
    )["address"]
    addr_recover2 = cli.create_account(
        recover2,
        coin_type=118,
        key_type="secp256k1",
    )["address"]
    sender = cli.address("community")
    transfer_via_cosmos(cli, sender, addr_recover1, amt)
    transfer_via_cosmos(cli, sender, addr_recover2, amt)
    do_multisig(cli, tmp_path, recover1, recover2, "multitest2")


def test_textual(evm):
    cli = evm.cosmos_cli()
    rsp = cli.transfer(
        cli.address("community"),
        cli.address("signer2"),
        f"1{DEFAULT_DENOM}",
        sign_mode="textual",
    )
    assert rsp["code"] == 0, rsp["raw_log"]


def test_key_src(evm, tmp_path):
    cli = evm.cosmos_cli()
    acct, mnemonic = Account.create_with_mnemonic(num_words=24)
    src = tmp_path / "mnemonic.txt"
    src.write_text(mnemonic)
    addr = cli.create_account("user", source=src)["address"]
    assert bech32_to_eth(addr) == acct.address


def test_comet_validator_set(evm):
    cli = evm.cosmos_cli()
    res = cli.comet_validator_set(cli.block_height())
    assert len(res["validators"]) == len(cli.validators())
