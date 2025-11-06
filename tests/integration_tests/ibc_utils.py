import json
import os
import subprocess
import tempfile
from contextlib import contextmanager
from pathlib import Path
from typing import NamedTuple

from pystarport import cluster, ports
from pystarport.utils import wait_for_new_blocks, wait_for_port

from .network import Evm, Hermes, setup_custom_evm
from .utils import (
    ADDRESS_PREFIX,
    CHAIN_ID,
    CMD,
    DEFAULT_DENOM,
    escrow_address,
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


def hermes_transfer(
    ibc,
    src_chain,
    src_key_name,
    src_amount,
    dst_chain,
    dst_addr,
    denom=DEFAULT_DENOM,
    memo=None,
    prefix=ADDRESS_PREFIX,
):
    port = "transfer"
    channel = "channel-0"
    # wait for hermes
    output = subprocess.getoutput(
        f"curl -s -X GET 'http://127.0.0.1:{ibc.hermes.port}/state' | jq"
    )
    assert json.loads(output)["status"] == "success"
    cmd = (
        f"hermes --config {ibc.hermes.configpath} tx ft-transfer "
        f"--dst-chain {dst_chain} --src-chain {src_chain} --src-port {port} "
        f"--src-channel {channel} --amount {src_amount} "
        f"--timeout-height-offset 1000 --number-msgs 1 "
        f"--denom {denom} --receiver {dst_addr} --key-name {src_key_name}"
    )
    if memo:
        cmd += f" --memo '{memo}'"
    subprocess.run(cmd, check=True, shell=True)
    return f"{port}/{channel}/{denom}", escrow_address(port, channel, prefix=prefix)
