import hashlib
import json
import math

import pytest
from eth_contract.erc20 import ERC20
from pystarport.utils import wait_for_fn, wait_for_fn_async

from .ibc_utils import hermes_transfer, prepare_network
from .utils import (
    ADDRS,
    DEFAULT_DENOM,
    KEYS,
    WETH_ADDRESS,
    assert_balance,
    assert_create_erc20_denom,
    build_and_deploy_contract_async,
    eth_to_bech32,
    find_duplicate,
    generate_isolated_address,
    ibc_denom_address,
    parse_events_rpc,
)

pytestmark = pytest.mark.asyncio


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    chain = request.config.getoption("chain_config")
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(path, name, chain)


def assert_dynamic_fee(cli):
    # assert that the relayer transactions do enables the dynamic fee extension option.
    criteria = "message.action='/ibc.core.channel.v1.MsgChannelOpenInit'"
    tx = cli.tx_search(criteria)["txs"][0]
    events = parse_events_rpc(tx["events"])
    fee = int(events["tx"]["fee"].removesuffix(DEFAULT_DENOM))
    gas = int(tx["gas_wanted"])
    # the effective fee is decided by the max_priority_fee (base fee is zero)
    # rather than the normal gas price
    cosmos_evm_dynamic_fee = 10000000000000000 / 10**18
    assert fee == math.ceil(gas * cosmos_evm_dynamic_fee)


def assert_dup_events(cli):
    # check duplicate OnRecvPacket events
    criteria = "message.action='/ibc.core.channel.v1.MsgRecvPacket'"
    events = cli.tx_search(criteria)["txs"][0]["events"]
    for event in events:
        dup = find_duplicate(event["attributes"])
        assert not dup, f"duplicate {dup} in {event['type']}"


def wait_for_balance_change(cli, addr, denom, init_balance):
    def check_balance():
        current_balance = cli.balance(addr, denom)
        return current_balance if current_balance != init_balance else None

    return wait_for_fn("balance change", check_balance)


def assert_receiver_events(cli, cli2, target):
    criteria = "message.action='/ibc.applications.transfer.v1.MsgTransfer'"
    events = cli.tx_search(criteria)["txs"][0]["events"]
    events = parse_events_rpc(events)
    receiver = events.get("ibc_transfer").get("receiver")
    assert receiver == target

    criteria = "message.action='/ibc.core.channel.v1.MsgRecvPacket'"
    events = cli2.tx_search(criteria)["txs"][0]["events"]
    events = parse_events_rpc(events)
    receiver = events.get("fungible_token_packet").get("receiver")
    assert receiver == target


async def test_ibc_transfer(ibc):
    w3 = ibc.ibc1.async_w3
    cli = ibc.ibc1.cosmos_cli()
    cli2 = ibc.ibc2.cosmos_cli()
    signer1 = ADDRS["signer1"]
    community = ADDRS["community"]
    addr_signer1 = eth_to_bech32(signer1)
    addr_community = eth_to_bech32(community)

    # evm-canary-net-2 signer2 -> evm-canary-net-1 signer1 100 baseunit
    transfer_amt = 100
    src_chain = "evm-canary-net-2"
    dst_chain = "evm-canary-net-1"
    path, escrow_addr = hermes_transfer(
        ibc,
        src_chain,
        "signer2",
        transfer_amt,
        dst_chain,
        addr_signer1,
    )
    denom_hash = hashlib.sha256(path.encode()).hexdigest().upper()
    dst_denom = f"ibc/{denom_hash}"
    signer1_balance_bf = cli.balance(addr_signer1, dst_denom)
    signer1_balance = wait_for_balance_change(
        cli, addr_signer1, dst_denom, signer1_balance_bf
    )
    assert signer1_balance == signer1_balance_bf + transfer_amt
    assert cli.ibc_denom_hash(path) == denom_hash
    assert_balance(cli2, ibc.ibc2.w3, escrow_addr) == transfer_amt
    assert_dynamic_fee(cli)
    assert_dup_events(cli)

    # evm-canary-net-1 signer1 -> evm-canary-net-2 community eth addr with 5 baseunit
    amount = 5
    rsp = cli.ibc_transfer(
        community,
        f"{amount}{DEFAULT_DENOM}",
        "channel-0",
        from_=addr_signer1,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    community_balance_bf = cli2.balance(addr_community, dst_denom)
    community_balance = wait_for_balance_change(
        cli2, addr_community, dst_denom, community_balance_bf
    )
    assert community_balance == community_balance_bf + amount
    assert_receiver_events(cli, cli2, community)

    ibc_erc20_addr = ibc_denom_address(dst_denom)
    assert (await ERC20.fns.decimals().call(w3, to=ibc_erc20_addr)) == 0
    total = await ERC20.fns.totalSupply().call(w3, to=ibc_erc20_addr)
    assert total == transfer_amt


async def prepare_dest_callback(w3, sender, amt):
    # deploy cb contract
    contract = await build_and_deploy_contract_async(
        w3, "CounterWithCallbacks", KEYS["signer1"]
    )
    calldata = await contract.functions.add(WETH_ADDRESS, amt).build_transaction(
        {"from": sender, "gas": 210000}
    )
    calldata = calldata["data"][2:]
    dest_cb = {
        "dest_callback": {
            "address": contract.address,
            "gas_limit": "1000000",
            "calldata": calldata,
        }
    }
    return contract.address, json.dumps(dest_cb)


async def test_ibc_cb(ibc):
    w3 = ibc.ibc1.async_w3
    cli = ibc.ibc1.cosmos_cli()
    cli2 = ibc.ibc2.cosmos_cli()
    signer1 = ADDRS["signer1"]
    signer2 = ADDRS["signer2"]
    addr_signer2 = eth_to_bech32(signer2)
    erc20_denom, total = await assert_create_erc20_denom(w3, signer1)

    # check native erc20 transfer
    res = cli.register_erc20(WETH_ADDRESS, _from="community", gas=400_000)
    assert res["code"] == 0
    erc20_denom = f"erc20:{WETH_ADDRESS}"
    res = cli.query_erc20_token_pair(erc20_denom)
    assert res["erc20_address"] == WETH_ADDRESS, res

    # evm-canary-net-1 signer1 -> evm-canary-net-2 signer2 50erc20_denom
    transfer_amt = total // 2
    src_chain = "evm-canary-net-1"
    dst_chain = "evm-canary-net-2"
    channel = "channel-0"
    isolated = generate_isolated_address(channel, addr_signer2)

    path, escrow_addr = hermes_transfer(
        ibc,
        src_chain,
        "signer1",
        transfer_amt,
        dst_chain,
        addr_signer2,
        denom=erc20_denom,
    )

    denom_hash = hashlib.sha256(path.encode()).hexdigest().upper()
    dst_denom = f"ibc/{denom_hash}"
    signer2_balance_bf = cli2.balance(addr_signer2, dst_denom)
    signer2_balance = wait_for_balance_change(
        cli2, addr_signer2, dst_denom, signer2_balance_bf
    )
    assert signer2_balance == signer2_balance_bf + transfer_amt
    assert cli2.ibc_denom_hash(path) == denom_hash
    signer2_balance_bf = signer2_balance

    assert cli.balance(escrow_addr, erc20_denom) == transfer_amt
    signer1_balance_eth = await ERC20.fns.balanceOf(signer1).call(w3, to=WETH_ADDRESS)
    assert signer1_balance_eth == total - transfer_amt

    # deploy cb contract
    transfer_amt = total // 2
    cb_contract, dest_cb = await prepare_dest_callback(w3, signer1, transfer_amt)
    cb_balance_bf = await ERC20.fns.balanceOf(cb_contract).call(w3, to=WETH_ADDRESS)

    # evm-canary-net-2 signer2 -> evm-canary-net-1 signer1 50erc20_denom
    src_chain = "evm-canary-net-2"
    dst_chain = "evm-canary-net-1"
    hermes_transfer(
        ibc,
        src_chain,
        "signer2",
        transfer_amt,
        dst_chain,
        isolated,
        denom=dst_denom,
        memo=dest_cb,
    )
    assert cli2.balance(addr_signer2, dst_denom) == signer2_balance_bf - transfer_amt

    async def wait_for_balance_change_async(w3, addr, token_addr, init_balance):
        async def check_balance():
            current_balance = await ERC20.fns.balanceOf(addr).call(w3, to=token_addr)
            return current_balance if current_balance != init_balance else None

        return await wait_for_fn_async("balance change", check_balance)

    cb_balance = await wait_for_balance_change_async(
        w3, cb_contract, WETH_ADDRESS, cb_balance_bf
    )
    assert cb_balance == cb_balance_bf + transfer_amt
    assert cli.balance(escrow_addr, erc20_denom) == 0
    assert cli2.balance(addr_signer2, dst_denom) == 0
