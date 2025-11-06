import json
import os
import time
from datetime import timedelta
from pathlib import Path

import pytest
import requests
import web3
from dateutil.parser import isoparse
from eth_account import Account
from eth_contract.contract import Contract
from pystarport import cluster
from pystarport.utils import (
    BondStatus,
    wait_for_block,
    wait_for_block_time,
    wait_for_new_blocks,
)

from .network import setup_custom_evm
from .utils import (
    ACCOUNTS,
    CMD,
    DEFAULT_DENOM,
    WEI_PER_DENOM,
    WEI_PER_ETH,
    address_to_bytes32,
    bech32_to_eth,
    build_contract,
    duration,
    edit_app_cfg,
    find_log_event_attrs,
)

PRECOMPILE = Contract(build_contract("StakingI")["abi"])
DELEGATE = PRECOMPILE.fns.delegate
UNDELEGATE = PRECOMPILE.fns.undelegate
VALIDATOR = PRECOMPILE.fns.validator
STAKING = "0x0000000000000000000000000000000000000800"
gas = 400_000


pytestmark = pytest.mark.asyncio


@pytest.fixture(scope="module")
def custom_evm(request, tmp_path_factory):
    chain = request.config.getoption("chain_config")
    path = tmp_path_factory.mktemp("staking")
    yield from setup_custom_evm(
        path,
        27200,
        Path(__file__).parent / "configs/staking.jsonnet",
        chain=chain,
    )


async def get_validators(w3):
    params = [b"", 0, 10, False, False]
    res, _ = await PRECOMPILE.fns.validators(BondStatus.BONDED.value, params).call(
        w3, to=STAKING
    )
    return res


async def test_staking_delegate(evm):
    cli = evm.cosmos_cli()
    w3 = evm.async_w3
    name = "signer1"
    amt = 2
    acct = ACCOUNTS[name]
    bonded = cli.staking_pool()
    balance_bf = await w3.eth.get_balance(acct.address)
    res = await get_validators(w3)
    addr = res[0][0]
    validator = cli.debug_addr(addr, bech="val")
    gas = 200_000
    res = await DELEGATE(acct.address, validator, amt).transact(
        w3, acct, to=STAKING, gas=gas
    )
    assert res.status == 1
    assert res.logs[0].topics == [
        PRECOMPILE.events.Delegate.topic,
        address_to_bytes32(acct.address),
        address_to_bytes32(addr),
    ]
    fee = res["gasUsed"] * res["effectiveGasPrice"]
    assert cli.staking_pool() == bonded + amt
    balance = await w3.eth.get_balance(acct.address)
    assert balance_bf == balance + amt * WEI_PER_DENOM + fee


async def test_staking_unbond(evm):
    cli = evm.cosmos_cli()
    unbond_duration = duration(cli.get_params("staking")["unbonding_time"])
    if unbond_duration > 60:
        pytest.skip(f"unbond_duration is {unbond_duration} too long for test")
    w3 = evm.async_w3
    name = "signer1"
    acct = ACCOUNTS[name]
    res = await get_validators(w3)
    val_ops = [cli.debug_addr(validator[0], bech="val") for validator in res[:2]]
    balance_bf = await w3.eth.get_balance(acct.address)
    bonded_bf = cli.staking_pool()
    amounts = [3, 4]
    fee = 0

    for i, amt in enumerate(amounts):
        res = await DELEGATE(acct.address, val_ops[i], amt).transact(
            w3, acct, to=STAKING, gas=gas
        )
        assert res.status == 1
        fee += res["gasUsed"] * res["effectiveGasPrice"]

    assert cli.staking_pool() == bonded_bf + sum(amounts)
    balance = await w3.eth.get_balance(acct.address)
    assert balance == balance_bf - sum(amounts) * WEI_PER_DENOM - fee

    unbonded_bf = cli.staking_pool(bonded=False)
    unbonded_amt = 2
    res = await UNDELEGATE(acct.address, val_ops[0], unbonded_amt).transact(
        w3, acct, to=STAKING, gas=gas
    )
    assert res.status == 1
    addr = cli.debug_addr(val_ops[0], bech="hex")
    assert res.logs[0].topics == [
        PRECOMPILE.events.Unbond.topic,
        address_to_bytes32(acct.address),
        address_to_bytes32(addr),
    ]
    fee += res["gasUsed"] * res["effectiveGasPrice"]
    assert cli.staking_pool(bonded=False) == unbonded_bf + unbonded_amt
    blk = res["blockNumber"]
    rsp = requests.get(f"{cli.node_rpc_http}/block_results?height={blk}").json()
    rsp = next((tx for tx in rsp["result"]["txs_results"] if tx["code"] == 0), None)
    data = find_log_event_attrs(
        rsp["events"], "unbond", lambda attrs: "completion_time" in attrs
    )
    wait_for_block_time(cli, isoparse(data["completion_time"]) + timedelta(seconds=1))
    balance = await w3.eth.get_balance(acct.address)
    assert balance == balance_bf - (sum(amounts) - unbonded_amt) * WEI_PER_DENOM - fee


async def test_staking_redelegate(evm):
    cli = evm.cosmos_cli()
    w3 = evm.async_w3
    name = "signer1"
    acct = ACCOUNTS[name]
    res = await get_validators(w3)
    val_ops = [cli.debug_addr(validator[0], bech="val") for validator in res[:2]]
    amounts = [3, 4]
    fee = 0

    for i, amt in enumerate(amounts):
        res = await DELEGATE(acct.address, val_ops[i], amt).transact(
            w3, acct, to=STAKING, gas=gas
        )
        assert res.status == 1
        fee += res["gasUsed"] * res["effectiveGasPrice"]

    DELEGATION = PRECOMPILE.fns.delegation
    _, balance_bf = await DELEGATION(acct.address, val_ops[0]).call(w3, to=STAKING)
    redelegate_amt = 2
    res = await PRECOMPILE.fns.redelegate(
        acct.address, val_ops[0], val_ops[1], redelegate_amt
    ).transact(w3, acct, to=STAKING, gas=gas)
    assert res.status == 1
    assert res.logs[0].topics == [
        PRECOMPILE.events.Redelegate.topic,
        address_to_bytes32(acct.address),
        address_to_bytes32(cli.debug_addr(val_ops[0], bech="hex")),
        address_to_bytes32(cli.debug_addr(val_ops[1], bech="hex")),
    ]
    fee += res["gasUsed"] * res["effectiveGasPrice"]
    _, balance = await DELEGATION(acct.address, val_ops[0]).call(w3, to=STAKING)
    assert balance_bf[1] == balance[1] + redelegate_amt


async def test_join_validator(evm):
    w3 = evm.async_w3
    mnemonic = os.getenv("VALIDATOR4_MNEMONIC")
    acct = Account.from_mnemonic(mnemonic)
    data = Path(evm.base_dir).parent
    chain_id = evm.config["chain_id"]
    clustercli = cluster.ClusterCLI(data, cmd=CMD, chain_id=chain_id)
    moniker = "new joined"
    node_index = clustercli.create_node(moniker=moniker, mnemonic=mnemonic)
    cli = clustercli.cosmos_cli(node_index)
    cli0 = evm.cosmos_cli()
    staked = 10_000_000_000_000_000_000
    fund = f"{staked + 1_000_000_000_000_000_000 // WEI_PER_DENOM}{DEFAULT_DENOM}"
    val_addr = cli.address("validator", bech="val")
    addr = cli0.debug_addr(val_addr, bech="hex")

    res = cli0.transfer(cli0.address("community"), cli.address("validator"), fund)
    assert res["code"] == 0, res
    edit_app_cfg(clustercli, node_index)
    clustercli.supervisor.startProcess(f"{chain_id}-node{node_index}")
    wait_for_block(cli, cli0.block_height() + 1)
    time.sleep(1)
    wait_for_block(cli, cli.block_height())

    count = len(cli.validators())
    pubkey = (
        cli.raw(
            "comet",
            "show-validator",
            home=cli.data_dir,
        )
        .strip()
        .decode()
    )
    pubkey = json.loads(pubkey)["key"]
    desc = [moniker, "identity", "website", "securityContact", "details"]
    commission = [
        int(0.1 * WEI_PER_ETH),
        int(0.2 * WEI_PER_ETH),
        int(0.01 * WEI_PER_ETH),
    ]
    min_self_delegation = 1
    res = await PRECOMPILE.fns.createValidator(
        desc, commission, min_self_delegation, acct.address, pubkey, staked
    ).transact(w3, acct, to=STAKING, gas=gas)
    assert res.status == 1
    assert res.logs[0].topics == [
        PRECOMPILE.events.CreateValidator.topic,
        address_to_bytes32(addr),
    ]
    time.sleep(2)
    assert len(cli.validators()) == count + 1

    val = cli.validator(val_addr)
    assert not val.get("jailed")
    assert val["status"] == BondStatus.BONDED.value
    assert val["tokens"] == str(staked)
    assert val["description"]["moniker"] == moniker
    assert val["commission"]["commission_rates"] == {
        "rate": "0.100000000000000000",
        "max_rate": "0.200000000000000000",
        "max_change_rate": "0.010000000000000000",
    }

    EDIT_VALIDATOR = PRECOMPILE.fns.editValidator
    msg = "commission cannot be changed more than once in 24h"
    with pytest.raises(web3.exceptions.ContractLogicError, match=msg):
        await EDIT_VALIDATOR(
            desc, acct.address, commission[0] * 2, min_self_delegation
        ).transact(w3, acct, to=STAKING)

    desc[0] = "awesome node"
    res = await EDIT_VALIDATOR(desc, acct.address, -1, -1).transact(
        w3, acct, to=STAKING, gas=gas
    )
    assert res.status == 1
    assert res.logs[0].topics == [
        PRECOMPILE.events.EditValidator.topic,
        address_to_bytes32(addr),
    ]
    assert cli.validator(val_addr)["description"]["moniker"] == "awesome node"


async def test_min_self_delegation(custom_evm):
    mnemonic = os.getenv("VALIDATOR4_MNEMONIC")
    acct = Account.from_mnemonic(mnemonic)
    cli = custom_evm.cosmos_cli(i=3)
    w3 = custom_evm.async_w3
    addr = bech32_to_eth(cli.address("validator"))
    val = cli.address("validator", bech="val")
    amt = 9_000_000_000_000_000_000
    res = await UNDELEGATE(acct.address, val, amt).transact(
        w3, acct, to=STAKING, gas=gas
    )
    assert res.status == 1
    res = await VALIDATOR(addr).call(w3, to=STAKING)
    assert res[3] == BondStatus.BONDED.to_int()
    amt = 1
    res = await UNDELEGATE(acct.address, val, amt).transact(
        w3, acct, to=STAKING, gas=gas
    )
    assert res.status == 1
    assert res.logs[0].topics == [
        PRECOMPILE.events.Unbond.topic,
        address_to_bytes32(acct.address),
        address_to_bytes32(addr),
    ]
    wait_for_new_blocks(cli, 2)
    res = await VALIDATOR(addr).call(w3, to=STAKING)
    assert res[3] == BondStatus.UNBONDING.to_int()
