import json
import math

import pytest
from eth_contract.erc20 import ERC20
from pystarport.utils import wait_for_fn_async

from .ibc_utils import (
    assert_hermes_transfer,
    assert_ibc_transfer,
    assert_receiver_events,
    prepare_network,
    run_hermes_transfer,
)
from .utils import (
    ADDRS,
    DEFAULT_DENOM,
    KEYS,
    WETH_ADDRESS,
    assert_create_erc20_denom,
    build_and_deploy_contract_async,
    escrow_address,
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


async def test_ibc_transfer(ibc):
    w3 = ibc.ibc1.async_w3
    cli = ibc.ibc1.cosmos_cli()
    cli2 = ibc.ibc2.cosmos_cli()
    signer1 = ADDRS["signer1"]
    community = ADDRS["community"]
    addr_signer1 = eth_to_bech32(signer1)

    # evm-canary-net-2 signer2 -> evm-canary-net-1 signer1 100atest
    transfer_amt = 100
    dst_denom, _ = assert_hermes_transfer(
        ibc.hermes,
        cli2,
        "signer2",
        transfer_amt,
        cli,
        addr_signer1,
    )
    assert_dynamic_fee(cli)
    assert_dup_events(cli)

    # evm-canary-net-1 signer1 -> evm-canary-net-2 community eth addr with 5atest
    amount = 5
    assert_ibc_transfer(
        ibc.hermes,
        cli,
        cli2,
        addr_signer1,
        community,
        amount,
        dst_denom,
    )
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
    port = "transfer"
    channel = "channel-0"
    isolated = generate_isolated_address(channel, addr_signer2)

    dst_denom, signer2_balance = assert_hermes_transfer(
        ibc.hermes,
        cli,
        "signer1",
        transfer_amt,
        cli2,
        addr_signer2,
        denom=erc20_denom,
        skip_src_balance_check=True,
    )

    signer1_balance_eth = await ERC20.fns.balanceOf(signer1).call(w3, to=WETH_ADDRESS)
    assert signer1_balance_eth == total - transfer_amt

    # deploy cb contract
    transfer_amt = total // 2
    cb_contract, dest_cb = await prepare_dest_callback(w3, signer1, transfer_amt)
    cb_balance_bf = await ERC20.fns.balanceOf(cb_contract).call(w3, to=WETH_ADDRESS)

    # evm-canary-net-2 signer2 -> evm-canary-net-1 signer1 50erc20_denom
    run_hermes_transfer(
        ibc.hermes,
        cli2,
        "signer2",
        transfer_amt,
        cli,
        isolated,
        denom=dst_denom,
        memo=dest_cb,
    )
    assert cli2.balance(addr_signer2, dst_denom) == signer2_balance - transfer_amt

    async def wait_for_balance_change_async(w3, addr, token_addr, init_balance):
        async def check_balance():
            current_balance = await ERC20.fns.balanceOf(addr).call(w3, to=token_addr)
            return current_balance if current_balance != init_balance else None

        return await wait_for_fn_async("balance change", check_balance)

    cb_balance = await wait_for_balance_change_async(
        w3, cb_contract, WETH_ADDRESS, cb_balance_bf
    )
    assert cb_balance == cb_balance_bf + transfer_amt
    escrow_addr = escrow_address(port, channel)
    assert cli.balance(escrow_addr, erc20_denom) == 0
    assert cli2.balance(addr_signer2, dst_denom) == 0
