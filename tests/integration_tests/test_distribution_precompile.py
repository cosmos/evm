import pytest
import requests
from eth_contract.contract import Contract
from pystarport.utils import parse_amount, wait_for_block, wait_for_new_blocks

from .utils import (
    ACCOUNTS,
    DEFAULT_DENOM,
    WEI_PER_DENOM,
    bech32_to_eth,
    build_contract,
    find_fee,
    find_log_event_attrs,
)

pytestmark = pytest.mark.asyncio

PRECOMPILE = Contract(build_contract("DistributionI")["abi"])

DISTRIBUTION = "0x0000000000000000000000000000000000000801"

gas = 400_000


async def community_pool(w3):
    res = await PRECOMPILE.fns.communityPool().call(w3, to=DISTRIBUTION)
    return res[0][1] if res else 0


async def rewards(w3, val, val_addr):
    res = await PRECOMPILE.fns.delegationRewards(val, val_addr).call(
        w3, to=DISTRIBUTION
    )
    return res[0][1] if res else 0


async def test_distribution(evm):
    cli = evm.cosmos_cli()
    tax = cli.get_params("distribution")["community_tax"]
    if float(tax) < 0.01:
        pytest.skip(f"community_tax is {tax} too low for test")
    w3 = evm.async_w3
    signer1, signer2 = cli.address("signer1"), cli.address("signer2")

    wait_for_block(cli, 2)

    balance_bf = cli.balance(signer1)
    community_bf = await community_pool(w3)

    amt = 2
    rsp = cli.transfer(signer1, signer2, f"{amt}{DEFAULT_DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]

    fee = find_fee(rsp)
    wait_for_new_blocks(cli, 2)

    assert cli.balance(signer1) == balance_bf - fee - amt
    assert await community_pool(w3) > community_bf


async def test_delegation_rewards_flow(evm):
    cli = evm.cosmos_cli()
    w3 = evm.async_w3
    acct = ACCOUNTS["signer1"]
    val = cli.validators()[0]["operator_address"]
    validator = cli.debug_addr(val, bech="hex")
    rewards_bf = await rewards(w3, validator, val)
    signer1 = cli.address("signer1")
    signer2 = cli.address("signer2")
    signer2_eth = bech32_to_eth(signer2)

    res = await PRECOMPILE.fns.setWithdrawAddress(acct.address, signer2).transact(
        w3, acct, to=DISTRIBUTION, gas=gas
    )
    assert res.status == 1

    delegate_amt = 4e6
    gas0 = 250_000
    coin = f"{delegate_amt}{DEFAULT_DENOM}"
    rsp = cli.delegate_amount(val, coin, _from=signer1, gas=gas0)
    assert rsp["code"] == 0, rsp["raw_log"]

    rewards_af = await rewards(w3, validator, val)
    assert rewards_af >= rewards_bf, "rewards should increase"

    balance_bf = await w3.eth.get_balance(signer2_eth)
    res = await PRECOMPILE.fns.withdrawDelegatorRewards(acct.address, val).transact(
        w3, acct, to=DISTRIBUTION, gas=gas
    )
    assert res.status == 1

    balance_af = await w3.eth.get_balance(signer2_eth)
    assert balance_af >= balance_bf, "balance should increase"

    rsp = cli.unbond_amount(val, coin, _from=signer1, gas=gas0)
    assert rsp["code"] == 0, rsp["raw_log"]


async def test_community_pool_funding(evm):
    w3 = evm.async_w3
    acct = ACCOUNTS["signer1"]
    initial_pool = await community_pool(w3)
    fund_amount = 1000
    balance_bf = await w3.eth.get_balance(acct.address)

    coin = [[DEFAULT_DENOM, fund_amount]]
    res = await PRECOMPILE.fns.fundCommunityPool(acct.address, coin).transact(
        w3, acct, to=DISTRIBUTION, gas=gas
    )
    assert res.status == 1

    balance_af = await w3.eth.get_balance(acct.address)
    fee = res["gasUsed"] * res["effectiveGasPrice"]
    assert (
        balance_af == balance_bf - fund_amount * WEI_PER_DENOM - fee
    ), "balance should decrease"

    final_pool = await community_pool(w3)
    assert final_pool >= initial_pool + fund_amount, "community pool should increase"


async def test_validator_rewards_pool_funding(evm):
    cli = evm.cosmos_cli()
    w3 = evm.async_w3
    acct = ACCOUNTS["signer1"]
    val = cli.validators()[0]["operator_address"]
    fund_amount = 1000
    coin = [[DEFAULT_DENOM, fund_amount]]
    res = await PRECOMPILE.fns.depositValidatorRewardsPool(
        acct.address, val, coin
    ).transact(w3, acct, to=DISTRIBUTION, gas=gas, check=False)
    # TODO: align disabled
    assert res.status == 1
    blk = res["blockNumber"]
    rsp = requests.get(f"{cli.node_rpc_http}/block_results?height={blk}").json()
    rsp = next((tx for tx in rsp["result"]["txs_results"] if tx["code"] == 0), None)
    data = find_log_event_attrs(
        rsp["events"], "rewards", lambda attrs: "amount" in attrs
    )
    assert parse_amount(data["amount"]) == fund_amount
