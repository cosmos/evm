import hashlib
import json
import math
import os
import subprocess
import tempfile
from contextlib import contextmanager
from pathlib import Path
from typing import NamedTuple

import tomlkit
from eth_contract.erc20 import ERC20
from pystarport import cluster, ports
from pystarport.utils import parse_amount, wait_for_new_blocks, wait_for_port

from .cosmoscli import CosmosCLI
from .network import Evm, Hermes, setup_custom_evm
from .utils import (
    ADDRESS_PREFIX,
    ADDRS,
    CHAIN_ID,
    CMD,
    DEFAULT_DENOM,
    escrow_address,
    find_duplicate,
    find_fee,
    ibc_denom_address,
    parse_events_rpc,
    wait_for_balance_change,
)


class IBCNetwork(NamedTuple):
    ibc1: Evm
    ibc2: Evm
    hermes: Hermes | None


def add_key(hermes, chain, mnemonic_env, key_name):
    with tempfile.NamedTemporaryFile("w", delete=False) as f:
        f.write(os.getenv(mnemonic_env))
        path = f.name
    try:
        subprocess.check_call(
            [
                "hermes",
                "--config",
                hermes.configpath,
                "keys",
                "add",
                "--hd-path",
                "m/44'/60'/0'/0/0",
                "--chain",
                chain,
                "--mnemonic-file",
                path,
                "--key-name",
                key_name,
                "--overwrite",
            ]
        )
    finally:
        os.unlink(path)


def call_hermes_cmd(hermes, incentivized, version, b_chain="evm-canary-net-2"):
    subprocess.check_call(
        [
            "hermes",
            "--config",
            hermes.configpath,
            "create",
            "channel",
            "--a-port",
            "transfer",
            "--b-port",
            "transfer",
            "--a-chain",
            CHAIN_ID,
            "--b-chain",
            b_chain,
            "--new-client-connection",
            "--yes",
        ]
        + (
            [
                "--channel-version",
                json.dumps(version),
            ]
            if incentivized
            else []
        )
    )
    add_key(hermes, CHAIN_ID, "SIGNER1_MNEMONIC", "signer1")
    add_key(hermes, b_chain, "SIGNER2_MNEMONIC", "signer2")


def prepare_network(tmp_path, name, chain, b_chain="evm-canary-net-2", cmd=CMD):
    name = f"configs/{name}.jsonnet"
    with contextmanager(setup_custom_evm)(
        tmp_path,
        27000,
        Path(__file__).parent / name,
        relayer=cluster.Relayer.HERMES.value,
        chain=chain,
    ) as ibc1:
        cli = ibc1.cosmos_cli()
        ibc2 = Evm(ibc1.base_dir.parent / b_chain, chain_binary=cmd)
        # wait for grpc ready
        wait_for_port(ports.grpc_port(ibc2.base_port(0)))
        wait_for_port(ports.grpc_port(ibc1.base_port(0)))
        wait_for_new_blocks(ibc2.cosmos_cli(), 1)
        wait_for_new_blocks(cli, 1)
        version = {"fee_version": "ics29-1", "app_version": "ics20-1"}
        path = ibc1.base_dir.parent / "relayer"
        hermes = Hermes(path.with_suffix(".toml"))
        call_hermes_cmd(hermes, False, version, b_chain=b_chain)
        ibc1.supervisorctl("start", "relayer-demo")
        yield IBCNetwork(ibc1, ibc2, hermes)
        wait_for_port(hermes.port)


def run_hermes_transfer(
    hermes: Hermes,
    src_cli: CosmosCLI,
    src_addr: str,
    src_amt: int,
    dst_cli: CosmosCLI,
    dst_addr: str,
    denom=DEFAULT_DENOM,
    memo=None,
    port="transfer",
    channel="channel-0",
):
    # wait for hermes
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{hermes.port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"
    src_chain = src_cli.chain_id
    dst_chain = dst_cli.chain_id
    cmd = (
        f"hermes --config {hermes.configpath} tx ft-transfer "
        f"--dst-chain {dst_chain} --src-chain {src_chain} --src-port {port} "
        f"--src-channel {channel} --amount {src_amt} "
        f"--timeout-height-offset 1000 --number-msgs 1 "
        f"--denom {denom} --receiver {dst_addr} --key-name {src_addr}"
    )
    if memo:
        cmd += f" --memo '{memo}'"
    subprocess.run(cmd, check=True, shell=True)


def assert_hermes_transfer(
    hermes: Hermes,
    src_cli: CosmosCLI,
    src_addr: str,
    src_amt: int,
    dst_cli: CosmosCLI,
    dst_addr: str,
    denom=DEFAULT_DENOM,
    memo=None,
    prefix=ADDRESS_PREFIX,
    port="transfer",
    channel="channel-0",
    skip_src_balance_check=False,
) -> tuple[str, str]:
    escrow_addr = escrow_address(port, channel, prefix=prefix)
    src_balance_bf = src_cli.balance(src_addr, denom)
    escrow_balance_bf = src_cli.balance(escrow_addr, denom)
    run_hermes_transfer(
        hermes,
        src_cli,
        src_addr,
        src_amt,
        dst_cli,
        dst_addr,
        denom,
        memo,
        port,
        channel,
    )
    fee = 0
    send_ibc_token = denom.startswith("ibc/")
    if send_ibc_token:
        dst_denom = src_cli.ibc_denom(denom).get("base")
    else:
        path = f"{port}/{channel}/{denom}"
        denom_hash = hashlib.sha256(path.encode()).hexdigest().upper()
        dst_denom = f"ibc/{denom_hash}"
        if should_deduct_fee(hermes, src_cli.chain_id, denom):
            fee = find_transfer_fee(src_cli)
    dst_balance_bf = dst_cli.balance(dst_addr, dst_denom)
    dst_balance = wait_for_balance_change(dst_cli, dst_addr, dst_denom, dst_balance_bf)
    assert dst_balance == dst_balance_bf + src_amt
    if not send_ibc_token:
        assert dst_cli.ibc_denom_hash(path) == denom_hash
        assert src_cli.balance(escrow_addr, denom) == escrow_balance_bf + src_amt
    if not skip_src_balance_check:
        assert src_cli.balance(src_addr, denom) == src_balance_bf - src_amt - fee
    return dst_denom, dst_balance


def should_deduct_fee(hermes: Hermes, chain_id, denom: str) -> bool:
    cfg = tomlkit.parse(hermes.configpath.read_text())
    for chain in cfg["chains"]:
        if chain["id"] == chain_id:
            return chain.get("gas_price", {}).get("denom") == denom
    return False


def assert_ibc_transfer(
    hermes: Hermes,
    src_cli,
    dst_cli: CosmosCLI,
    src_addr,
    dst_eth_addr: str,
    amt: int,
    dst_denom: str,
    denom=DEFAULT_DENOM,
    channel="channel-0",
    **kwargs,
) -> None:
    src_balance_bf = src_cli.balance(src_addr, denom=denom)
    rsp = src_cli.ibc_transfer(
        dst_eth_addr, f"{amt}{denom}", channel, from_=src_addr, **kwargs
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    fee = find_fee(rsp) if should_deduct_fee(hermes, src_cli.chain_id, denom) else 0
    dst_addr = dst_cli.debug_addr(dst_eth_addr, bech="acc")
    dst_balance_bf = dst_cli.balance(dst_addr, dst_denom)
    dst_balance = wait_for_balance_change(dst_cli, dst_addr, dst_denom, dst_balance_bf)
    assert dst_balance == dst_balance_bf + amt
    assert src_cli.balance(src_addr, denom=denom) == src_balance_bf - amt - fee


def ibc_denom_hash(path):
    return hashlib.sha256(path.encode()).hexdigest().upper()


def find_transfer_fee(cli):
    criteria = "message.action='/ibc.applications.transfer.v1.MsgTransfer'"
    tx = cli.tx_search(criteria, order_by="desc", limit=1)["txs"][0]
    events = parse_events_rpc(tx["events"])
    return int(parse_amount(events["tx"]["fee"]))


def assert_receiver_events(cli, cli2: CosmosCLI, dst_eth_addr: str):
    criteria = "message.action='/ibc.applications.transfer.v1.MsgTransfer'"
    events = cli.tx_search(criteria, order_by="desc", limit=1)["txs"][0]
    events = parse_events_rpc(events["events"])
    assert events.get("ibc_transfer").get("receiver") == dst_eth_addr

    criteria = "message.action='/ibc.core.channel.v1.MsgRecvPacket'"
    events = cli2.tx_search(criteria, order_by="desc", limit=1)["txs"][0]
    events = parse_events_rpc(events["events"])
    assert events.get("fungible_token_packet").get("receiver") == dst_eth_addr


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


async def assert_ibc_transfer_flow(
    ibc: IBCNetwork,
    denom=DEFAULT_DENOM,
    chain2_denom="atest",
    chain2_prefix="cosmos",
    return_ratio=1,
) -> str:
    w3 = ibc.ibc1.async_w3
    cli1 = ibc.ibc1.cosmos_cli()
    cli2 = ibc.ibc2.cosmos_cli()
    chain2_gas_prices = f"10000000000{chain2_denom}"
    amt = 100
    amt2 = 2
    amt3 = 3
    amt4 = 4
    port = "transfer"
    channel = "channel-0"
    eth_community = ADDRS["community"]
    for c in [cli1, cli2]:
        add_key(ibc.hermes, c.chain_id, "COMMUNITY_MNEMONIC", "community")

    print(f"chain2 signer2 -> chain1 signer1 {amt}{chain2_denom}")
    dst_denom, _ = assert_hermes_transfer(
        ibc.hermes,
        cli2,
        "signer2",
        amt,
        cli1,
        cli1.address("signer1"),
        denom=chain2_denom,
        prefix=chain2_prefix,
    )
    assert_dynamic_fee(cli1)
    assert_dup_events(cli1)
    ibc_erc20_addr = ibc_denom_address(dst_denom)
    assert (await ERC20.fns.decimals().call(w3, to=ibc_erc20_addr)) == 0
    assert await ERC20.fns.totalSupply().call(w3, to=ibc_erc20_addr) == amt

    print(f"chain1 signer1 -> chain2 signer2 {amt2}{denom}")
    dst_denom2, _ = assert_hermes_transfer(
        ibc.hermes,
        cli1,
        "signer1",
        amt2,
        cli2,
        cli2.address("signer2"),
        denom=denom,
    )

    print(f"chain1 community -> chain2 eth_community {amt3}{denom}")
    denom_hash = ibc_denom_hash(f"{port}/{channel}/{denom}")
    dst_denom3 = f"ibc/{denom_hash}"
    assert_ibc_transfer(
        ibc.hermes,
        cli1,
        cli2,
        "community",
        eth_community,
        amt3,
        dst_denom3,
        denom=denom,
    )
    assert_receiver_events(cli1, cli2, eth_community)

    print(f"chain2 community -> chain1 eth_community {amt4}{chain2_denom}")
    denom_hash = ibc_denom_hash(f"{port}/{channel}/{chain2_denom}")
    dst_denom4 = f"ibc/{denom_hash}"
    assert_ibc_transfer(
        ibc.hermes,
        cli2,
        cli1,
        "community",
        eth_community,
        amt4,
        dst_denom4,
        denom=chain2_denom,
        gas_prices=chain2_gas_prices,
    )
    assert_receiver_events(cli2, cli1, eth_community)

    amt = int(amt * return_ratio)
    print(f"chain1 signer1 -> chain2 signer2 back {amt}{dst_denom}")
    assert_hermes_transfer(
        ibc.hermes,
        cli1,
        "signer1",
        amt,
        cli2,
        cli2.address("signer2"),
        denom=dst_denom,
    )

    amt2 = int(amt2 * return_ratio)
    print(f"chain2 signer2 -> chain1 signer1 back {amt2}{dst_denom2}")
    assert_hermes_transfer(
        ibc.hermes,
        cli2,
        "signer2",
        amt2,
        cli1,
        cli1.address("signer1"),
        denom=dst_denom2,
        prefix=chain2_prefix,
    )

    amt3 = int(amt3 * return_ratio)
    print(f"chain2 community -> chain1 eth_community back {amt3}{dst_denom3}")
    assert_ibc_transfer(
        ibc.hermes,
        cli2,
        cli1,
        "community",
        eth_community,
        amt3,
        denom,
        denom=dst_denom3,
        gas_prices=chain2_gas_prices,
    )
    assert_receiver_events(cli2, cli1, eth_community)

    amt4 = int(amt4 * return_ratio)
    print(f"chain1 community -> chain2 eth_community back {amt4}{dst_denom4}")
    assert_ibc_transfer(
        ibc.hermes,
        cli1,
        cli2,
        "community",
        eth_community,
        amt4,
        chain2_denom,
        denom=dst_denom4,
    )
    assert_receiver_events(cli1, cli2, eth_community)
    return ibc_erc20_addr
