import pytest
import web3
from eth_contract.utils import send_transaction
from web3 import AsyncWeb3

from .utils import (
    ACCOUNTS,
    ADDRS,
    KEYS,
    AsyncGreeter,
    w3_wait_for_new_blocks_async,
)

pytestmark = pytest.mark.asyncio


async def test_get_logs_by_topic(evm):
    w3: AsyncWeb3 = evm.async_w3
    greeter = AsyncGreeter(key=KEYS["community"])
    addr = await greeter.deploy(w3)
    contract = greeter.contract
    topic = contract.events.ChangeGreeting.topic
    res = await greeter.set_greeting("Hello")
    current = await w3.eth.block_number
    # invalid block ranges
    test_cases = [
        {"fromBlock": hex(2000), "toBlock": "latest", "address": [addr]},
        {"fromBlock": hex(2), "toBlock": hex(1), "address": [addr]},
        {
            "fromBlock": "earliest",
            "toBlock": hex(current + 200),
            "address": [addr],
        },
        {
            "fromBlock": hex(current + 20),
            "toBlock": hex(current + 200),
            "address": [addr],
        },
    ]
    invalid_block_msg = "invalid block range params"
    for params in test_cases:
        with pytest.raises(web3.exceptions.Web3RPCError, match=invalid_block_msg):
            await w3.eth.get_logs(params)

    # log exists
    logs = await w3.eth.get_logs({"topics": [topic]})
    assert len(logs) == 1
    log = logs[0]
    assert all(
        log[key] == res[key]
        for key in ["transactionHash", "transactionIndex", "blockNumber", "blockHash"]
    )
    assert log["address"] == addr
    assert log["blockTimestamp"] == res["logs"][0]["blockTimestamp"]
    assert log["blockTimestamp"] != "0x0"

    # Wait and confirm log doesn't appear in new blocks
    await w3_wait_for_new_blocks_async(w3, 2)
    assert len(await w3.eth.get_logs({"topics": [topic]})) == 0

    previous = current
    current = await w3.eth.block_number
    # valid block ranges
    valid_cases = [
        {"fromBlock": "earliest", "toBlock": "latest", "address": [addr]},
        {
            "fromBlock": "earliest",
            "toBlock": hex(current),
            "address": [addr],
        },
        {
            "fromBlock": hex(previous),
            "toBlock": "latest",
            "address": [addr],
        },
        {
            "fromBlock": hex(previous),
            "toBlock": hex(current),
            "address": [addr],
        },
    ]
    for params in valid_cases:
        logs = await w3.eth.get_logs(params)
        assert len(logs) > 0


async def test_pending_transaction_filter(evm):
    w3: AsyncWeb3 = evm.async_w3
    flt = await w3.eth.filter("pending")
    assert await flt.get_new_entries() == []
    tx = {"to": ADDRS["signer1"], "value": 1000}
    receipt = await send_transaction(w3, ACCOUNTS["community"], **tx)
    assert receipt.status == 1
    assert receipt["transactionHash"] in await flt.get_new_entries()


async def test_block_filter(evm):
    w3: AsyncWeb3 = evm.async_w3
    flt = await w3.eth.filter("latest")
    # new blocks
    await w3_wait_for_new_blocks_async(w3, 1)
    tx = {"to": ADDRS["signer1"], "value": 1000}
    receipt = await send_transaction(w3, ACCOUNTS["community"], **tx)
    assert receipt.status == 1
    blocks = await flt.get_new_entries()
    assert len(blocks) >= 1


async def test_event_log_filter(evm):
    w3: AsyncWeb3 = evm.async_w3
    greeter = AsyncGreeter(key=KEYS["community"])
    await greeter.deploy(w3)
    contract = greeter.contract
    assert "Hello" == await greeter.greet()
    current_height = hex(await w3.eth.get_block_number())
    event_filter = await contract.events.ChangeGreeting.create_filter(
        w3, from_block=current_height
    )
    res = await greeter.set_greeting("world")
    log = contract.events.ChangeGreeting().process_receipt(res)[0]
    assert log["event"] == "ChangeGreeting"
    new_entries = await event_filter.get_new_entries()
    assert len(new_entries) == 1
    assert new_entries[0] == log
    assert "world" == await greeter.greet()
    # without new txs since last call``
    assert await event_filter.get_new_entries() == []
    assert await event_filter.get_all_entries() == new_entries
    # Uninstall
    assert await w3.eth.uninstall_filter(event_filter.filter_id)
    assert not await w3.eth.uninstall_filter(event_filter.filter_id)
