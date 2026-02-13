import json

import pytest
from eth_contract.erc20 import ERC20
from pystarport.utils import wait_for_fn_async
from web3 import AsyncWeb3

from .ibc_utils import (
    assert_hermes_transfer,
    assert_ibc_transfer_flow,
    prepare_network,
    run_hermes_transfer,
)
from .utils import (
    ADDRESS_PREFIX,
    ADDRS,
    DEFAULT_DENOM,
    KEYS,
    WETH_ADDRESS,
    assert_create_erc20_denom,
    build_and_deploy_contract_async,
    escrow_address,
    eth_to_bech32,
    generate_isolated_address,
)

pytestmark = pytest.mark.asyncio


@pytest.fixture(scope="module")
def ibc(request, tmp_path_factory):
    "prepare-network"
    name = "ibc"
    chain = request.config.getoption("chain_config")
    path = tmp_path_factory.mktemp(name)
    yield from prepare_network(path, name, chain)


async def wait_for_balance_change_async(
    w3: AsyncWeb3, addr, token_addr: str, init_balance: int
):
    async def check_balance():
        current_balance = await ERC20.fns.balanceOf(addr).call(w3, to=token_addr)
        return current_balance if current_balance != init_balance else None

    return await wait_for_fn_async("balance change", check_balance)


async def test_ibc_transfer(ibc):
    await assert_ibc_transfer_flow(
        ibc,
        chain2_denom=DEFAULT_DENOM,
        chain2_prefix=ADDRESS_PREFIX,
        return_ratio=0.5,
    )


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

    transfer_amt = total // 2
    print(f"chain1 signer1 -> chain2 signer2 {transfer_amt}{erc20_denom}")
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

    print(f"chain2 signer2 -> chain1 signer1 {transfer_amt}{dst_denom}")
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
    cb_balance = await wait_for_balance_change_async(
        w3, cb_contract, WETH_ADDRESS, cb_balance_bf
    )
    assert cb_balance == cb_balance_bf + transfer_amt
    escrow_addr = escrow_address(port, channel)
    assert cli.balance(escrow_addr, erc20_denom) == 0
    assert cli2.balance(addr_signer2, dst_denom) == 0
