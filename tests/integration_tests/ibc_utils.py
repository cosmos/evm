import hashlib
import json
import os
import subprocess
import tempfile
from contextlib import contextmanager
from pathlib import Path
from typing import NamedTuple

import tomlkit
from pystarport import cluster, ports
from pystarport.utils import parse_amount, wait_for_new_blocks, wait_for_port

from .cosmoscli import CosmosCLI
from .network import Evm, Hermes, setup_custom_evm
from .utils import (
    ADDRESS_PREFIX,
    CHAIN_ID,
    CMD,
    DEFAULT_DENOM,
    escrow_address,
    find_fee,
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
