import time
from datetime import timedelta
from pathlib import Path

import pytest
from dateutil.parser import isoparse
from pystarport import cluster
from pystarport.utils import (
    BondStatus,
    wait_for_block,
    wait_for_block_time,
    wait_for_new_blocks,
)

from .network import setup_custom_evm
from .utils import (
    CMD,
    DEFAULT_DENOM,
    DEFAULT_GAS_PRICE,
    WEI_PER_DENOM,
    duration,
    edit_app_cfg,
    find_fee,
    find_log_event_attrs,
)

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def custom_evm(request, tmp_path_factory):
    chain = request.config.getoption("chain_config")
    path = tmp_path_factory.mktemp("staking")
    yield from setup_custom_evm(
        path,
        26800,
        Path(__file__).parent / "configs/staking.jsonnet",
        chain=chain,
    )


def test_staking_unbond(evm):
    cli = evm.cosmos_cli()
    unbond_duration = duration(cli.get_params("staking")["unbonding_time"])
    if unbond_duration > 60:
        pytest.skip(f"unbond_duration is {unbond_duration} too long for test")
    name = "signer1"
    signer1 = cli.address(name)
    validators = cli.validators()
    val_ops = [v["operator_address"] for v in validators[:2]]
    balance_bf = cli.balance(signer1)
    bonded_bf = cli.staking_pool()
    amounts = [3, 4]
    fee = 0
    gas = 250_000

    for i, amt in enumerate(amounts):
        rsp = cli.delegate_amount(
            val_ops[i], f"{amt}{DEFAULT_DENOM}", _from=name, gas=gas
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        fee += find_fee(rsp)

    assert cli.staking_pool() == bonded_bf + sum(amounts)
    assert cli.balance(signer1) == balance_bf - sum(amounts) - fee

    unbonded = cli.staking_pool(bonded=False)
    unbonded_amt = 2
    rsp = cli.unbond_amount(
        val_ops[1], f"{unbonded_amt}{DEFAULT_DENOM}", _from=name, gas=gas
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    fee += find_fee(rsp)
    assert cli.staking_pool(bonded=False) == unbonded + unbonded_amt
    data = find_log_event_attrs(
        rsp["events"], "unbond", lambda attrs: "completion_time" in attrs
    )
    wait_for_block_time(cli, isoparse(data["completion_time"]) + timedelta(seconds=1))
    assert cli.balance(signer1) == balance_bf - (sum(amounts) - unbonded_amt) - fee


def test_staking_redelegate(evm):
    cli = evm.cosmos_cli()
    name = "signer1"
    signer1 = cli.address(name)
    validators = cli.validators()
    val_ops = [v["operator_address"] for v in validators[:2]]
    amounts = [3, 4]
    fee = 0
    gas = 400_000

    for i, amt in enumerate(amounts):
        rsp = cli.delegate_amount(
            val_ops[i], f"{amt}{DEFAULT_DENOM}", _from=name, gas=gas
        )
        assert rsp["code"] == 0, rsp["raw_log"]
        fee += find_fee(rsp)

    balance_bf = cli.delegation(signer1, val_ops[0])["balance"]["amount"]
    redelegate_amt = 2
    rsp = cli.redelegate(
        val_ops[0],
        val_ops[1],
        f"{redelegate_amt}{DEFAULT_DENOM}",
        _from=name,
        gas=gas,
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    balance = cli.delegation(signer1, val_ops[0])["balance"]["amount"]
    assert int(balance_bf) == int(balance) + redelegate_amt


def test_join_validator(evm):
    data = Path(evm.base_dir).parent
    chain_id = evm.config["chain_id"]
    clustercli = cluster.ClusterCLI(data, cmd=CMD, chain_id=chain_id)
    moniker = "new joined"
    node_index = clustercli.create_node(moniker=moniker)
    cli = clustercli.cosmos_cli(node_index)
    cli0 = evm.cosmos_cli()
    staked = 10_000_000_000_000_000_000
    fund = f"{staked + 1_000_000_000_000_000_000 // WEI_PER_DENOM}{DEFAULT_DENOM}"
    val_addr = cli.address("validator", bech="val")
    addr = cli.address("validator")
    res = cli0.transfer(cli0.address("community"), addr, fund)
    assert res["code"] == 0, res
    edit_app_cfg(clustercli, node_index)
    clustercli.supervisor.startProcess(f"{chain_id}-node{node_index}")
    wait_for_block(cli, cli0.block_height() + 1)
    time.sleep(1)
    wait_for_block(cli, cli.block_height())

    count = len(cli.validators())
    gas = 420_000
    opts = {"gas_prices": DEFAULT_GAS_PRICE, "gas": gas}
    rsp = cli.create_validator(f"{staked}{DEFAULT_DENOM}", {"moniker": moniker}, **opts)
    assert rsp["code"] == 0, rsp["raw_log"]
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
    rsp = cli.edit_validator(commission_rate="0.2", **opts)
    assert rsp["code"] == 12, rsp["raw_log"]
    assert "commission cannot be changed more than once in 24h" in rsp["raw_log"]
    rsp = cli.edit_validator(new_moniker="awesome node", **opts)
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cli.validator(val_addr)["description"]["moniker"] == "awesome node"


def test_min_self_delegation(custom_evm):
    cli = custom_evm.cosmos_cli(i=3)
    val = cli.address("validator", bech="val")
    addr = cli.address("validator")
    gas = 320_000
    amt = 9_000_000_000_000_000_000
    assert (
        cli.unbond_amount(val, f"{amt}{DEFAULT_DENOM}", _from=addr, gas=gas)["code"]
        == 0
    )
    assert cli.validator(val).get("status") == BondStatus.BONDED.value
    assert cli.unbond_amount(val, f"1{DEFAULT_DENOM}", _from=addr, gas=gas)["code"] == 0
    wait_for_new_blocks(cli, 2)
    assert cli.validator(val).get("status") == BondStatus.UNBONDING.value
