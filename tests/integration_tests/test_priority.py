import sys
from pathlib import Path

import pytest
import web3
from pystarport.utils import wait_for_new_blocks

from .network import setup_custom_evm
from .utils import (
    ADDRS,
    DEFAULT_DENOM,
    KEYS,
    WEI_PER_DENOM,
    eth_to_bech32,
    send_transaction,
    sign_transaction,
)

PRIORITY_REDUCTION = 1000000


def effective_gas_price(tx, base_fee):
    if "maxFeePerGas" in tx:
        # dynamic fee tx
        return min(base_fee + tx["maxPriorityFeePerGas"], tx["maxFeePerGas"])
    else:
        # legacy tx
        return tx["gasPrice"]


def tx_priority(tx, base_fee):
    if "maxFeePerGas" in tx:
        # dynamic fee tx
        return (
            min(tx["maxPriorityFeePerGas"], tx["maxFeePerGas"] - base_fee)
            // PRIORITY_REDUCTION
        )
    else:
        # legacy tx
        return (tx["gasPrice"] - base_fee) // PRIORITY_REDUCTION


def test_priority(evm):
    """
    test priorities of different tx types

    use a relatively large priority number to counter
    the effect of base fee change during the testing.
    """
    w3 = evm.w3
    amount = 10000
    base_fee = w3.eth.get_block("latest").baseFeePerGas

    # [ ( sender, tx ), ... ]
    # use different senders to avoid nonce conflicts
    test_cases = [
        (
            "validator",
            {
                "to": "0x0000000000000000000000000000000000000000",
                "value": amount,
                "gas": 21000,
                "maxFeePerGas": base_fee + PRIORITY_REDUCTION * 600000,
                "maxPriorityFeePerGas": PRIORITY_REDUCTION,
            },
        ),
        (
            "community",
            {
                "to": "0x0000000000000000000000000000000000000000",
                "value": amount,
                "gas": 21000,
                "gasPrice": base_fee + PRIORITY_REDUCTION * 200000,
            },
        ),
        (
            "signer2",
            {
                "to": "0x0000000000000000000000000000000000000000",
                "value": amount,
                "gasPrice": base_fee + PRIORITY_REDUCTION * 400000,
                "accessList": [
                    {
                        "address": "0xde0b295669a9fd93d5f28d9ec85e40f4cb697bae",
                        "storageKeys": (
                            "0x00000000000000000000000000000000000000000000000000000000"
                            "00000003",
                            "0x00000000000000000000000000000000000000000000000000000000"
                            "00000007",
                        ),
                    }
                ],
            },
        ),
        (
            "signer1",
            {
                "to": "0x0000000000000000000000000000000000000000",
                "value": amount,
                "gas": 21000,
                "maxFeePerGas": base_fee + PRIORITY_REDUCTION * 600000,
                "maxPriorityFeePerGas": PRIORITY_REDUCTION * 600000,
            },
        ),
    ]

    # test cases are ordered by priority
    expect_priorities = [tx_priority(tx, base_fee) for _, tx in test_cases]
    assert expect_priorities == [1, 200000, 400000, 600000]

    signed = [sign_transaction(w3, tx, key=KEYS[sender]) for sender, tx in test_cases]
    # send the txs from low priority to high,
    # but the later sent txs should be included earlier.
    txhashes = [w3.eth.send_raw_transaction(tx.raw_transaction) for tx in signed]

    receipts = [w3.eth.wait_for_transaction_receipt(txhash) for txhash in txhashes]
    print(receipts)
    assert all(receipt.status == 1 for receipt in receipts), "expect all txs success"

    # the later txs should be included earlier because of higher priority
    # FIXME there's some non-deterministics due to mempool logic
    tx_indexes = [(r.blockNumber, r.transactionIndex) for r in receipts]
    print(tx_indexes)
    # the first sent tx are included later, because of lower priority
    # ensure desc within continuous block
    assert all(
        (b1 < b2 or (b1 == b2 and i1 > i2))
        for (b1, i1), (b2, i2) in zip(tx_indexes, tx_indexes[1:])
    )


def conver_gas_prices(base_fee, value):
    return base_fee + (value * PRIORITY_REDUCTION) / WEI_PER_DENOM


@pytest.fixture(scope="module")
def custom_evm(request, tmp_path_factory):
    chain = request.config.getoption("chain_config")
    path = tmp_path_factory.mktemp("default")
    yield from setup_custom_evm(
        path,
        27100,
        Path(__file__).parent / "configs/default.jsonnet",
        chain=chain,
    )


def test_native_tx_priority(custom_evm):
    cli = custom_evm.cosmos_cli()
    base_fee = float(cli.query_base_fee())
    amt = f"1000{DEFAULT_DENOM}"
    test_cases = [
        {
            "from": eth_to_bech32(ADDRS["community"]),
            "to": eth_to_bech32(ADDRS["validator"]),
            "amount": amt,
            "gas_prices": f"{conver_gas_prices(base_fee, 600000)}{DEFAULT_DENOM}",
            "max_priority_price": 0,
        },
        {
            "from": eth_to_bech32(ADDRS["signer1"]),
            "to": eth_to_bech32(ADDRS["signer2"]),
            "amount": amt,
            "gas_prices": f"{conver_gas_prices(base_fee, 600000)}{DEFAULT_DENOM}",
            "max_priority_price": PRIORITY_REDUCTION * 200000 / WEI_PER_DENOM,
        },
        {
            "from": eth_to_bech32(ADDRS["signer2"]),
            "to": eth_to_bech32(ADDRS["signer1"]),
            "amount": amt,
            "gas_prices": f"{conver_gas_prices(base_fee, 400000)}{DEFAULT_DENOM}",
            "max_priority_price": PRIORITY_REDUCTION * 400000 / WEI_PER_DENOM,
        },
        {
            "from": eth_to_bech32(ADDRS["validator"]),
            "to": eth_to_bech32(ADDRS["community"]),
            "amount": amt,
            "gas_prices": f"{conver_gas_prices(base_fee, 600000)}{DEFAULT_DENOM}",
            "max_priority_price": None,  # no extension, maximum tipFeeCap
        },
    ]
    txs = []
    expect_priorities = []
    for tc in test_cases:
        tx = cli.transfer(
            tc["from"],
            tc["to"],
            tc["amount"],
            gas_prices=tc["gas_prices"],
            generate_only=True,
        )
        txs.append(
            cli.sign_tx_json(
                tx, tc["from"], max_priority_price=tc.get("max_priority_price")
            )
        )
        gas_price = float(tc["gas_prices"].removesuffix(DEFAULT_DENOM))
        res = min(
            get_max_priority_price(tc.get("max_priority_price")), gas_price - base_fee
        )
        expect_priorities.append((res * WEI_PER_DENOM) // PRIORITY_REDUCTION)
    assert expect_priorities == [0, 200000, 400000, 600000]

    txhashes = []
    for tx in txs:
        rsp = cli.broadcast_tx_json(tx)
        assert rsp["code"] == 0, rsp["raw_log"]
        txhashes.append(rsp["txhash"])

    print("wait for two new blocks, so the sent txs are all included")
    wait_for_new_blocks(cli, 2)

    tx_results = [cli.tx_search_rpc(f"tx.hash='{txhash}'")[0] for txhash in txhashes]
    tx_indexes = [(int(r["height"]), r["index"]) for r in tx_results]
    print(tx_indexes)
    # the first sent tx are included later, because of lower priority
    # ensure desc within continuous block
    assert all(
        (b1 < b2 or (b1 == b2 and i1 > i2))
        for (b1, i1), (b2, i2) in zip(tx_indexes, tx_indexes[1:])
    )


def get_max_priority_price(max_priority_price):
    "default to max int64 if None"
    return max_priority_price if max_priority_price is not None else sys.maxsize


def test_validate(evm):
    w3 = evm.w3
    gas = int(1.2 * w3.eth.gas_price)
    tx = {
        "to": "0x0000000000000000000000000000000000000000",
        "value": 1,
        "gas": 21000,
        "maxFeePerGas": gas,
        "maxPriorityFeePerGas": gas + 1,
    }
    with pytest.raises(web3.exceptions.Web3RPCError) as exc:
        send_transaction(w3, tx)
    assert "max priority fee per gas higher than max fee per gas" in str(exc)
