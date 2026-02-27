import json
import subprocess

import requests
from pystarport.cosmoscli import CosmosCLI as PystarportCosmosCLI
from pystarport.utils import build_cli_args_safe, interact

from .utils import (
    DEFAULT_DENOM,
    DEFAULT_GAS,
    DEFAULT_GAS_PRICE,
    MNEMONICS,
)


class ChainCommand:
    def __init__(self, cmd):
        self.cmd = cmd

    def __call__(self, cmd, *args, stdin=None, stderr=subprocess.STDOUT, **kwargs):
        "execute evmd"
        args = " ".join(build_cli_args_safe(cmd, *args, **kwargs))
        return interact(f"{self.cmd} {args}", input=stdin, stderr=stderr)


class CosmosCLI(PystarportCosmosCLI):
    "the apis to interact with wallet and blockchain"

    def __init__(
        self,
        data_dir,
        node_rpc,
        cmd,
        chain_id=None,
        gas=DEFAULT_GAS,
        gas_prices=DEFAULT_GAS_PRICE,
    ):
        super().__init__(data_dir, node_rpc, chain_id, cmd, gas, gas_prices)
        self.raw = ChainCommand(cmd)
        genesis_path = self.data_dir / "config" / "genesis.json"
        if genesis_path.exists():
            self._genesis = json.loads(genesis_path.read_text())
            if chain_id is None:
                self.chain_id = self._genesis["chain_id"]
        else:
            self._genesis = {}
            if chain_id is not None:
                # avoid client.yml overwrite flag in textual mode
                self.raw(
                    "config", "set", "client", "chain-id", chain_id, home=self.data_dir
                )
                self.raw(
                    "config", "set", "client", "node", node_rpc, home=self.data_dir
                )

    @property
    def node_rpc_http(self):
        url = self.node_rpc.removeprefix("tcp")
        if not url.startswith(("http://", "https://")):
            url = "http" + url
        return url

    @classmethod
    def init(cls, moniker, data_dir, node_rpc, cmd, chain_id):
        "the node's config is already added"
        ChainCommand(cmd)(
            "init",
            moniker,
            chain_id=chain_id,
            home=data_dir,
        )
        return cls(data_dir, node_rpc, cmd)

    def balance(self, addr, denom=DEFAULT_DENOM, height=0):
        return super().balance(addr, denom=denom, height=height)

    def address(self, name, bech="acc", field="address", skip_create=False):
        try:
            output = self.raw(
                "keys",
                "show",
                name,
                f"--{field}",
                home=self.data_dir,
                keyring_backend="test",
                bech=bech,
            )
        except AssertionError as e:
            if skip_create:
                raise
            if "not a valid name or address" in str(e):
                self.create_account(name, mnemonic=MNEMONICS[name], home=self.data_dir)
                output = self.raw(
                    "keys",
                    "show",
                    name,
                    f"--{field}",
                    home=self.data_dir,
                    keyring_backend="test",
                    bech=bech,
                )
            else:
                raise
        return output.strip().decode()

    def debug_addr(self, eth_addr, bech="acc"):
        output = self.raw("debug", "addr", eth_addr).decode().strip().split("\n")
        if bech == "val":
            prefix = "Bech32 Val"
        elif bech == "hex":
            prefix = "Address hex:"
        else:
            prefix = "Bech32 Acc"
        for line in output:
            if line.startswith(prefix):
                return line.split()[-1]
        return eth_addr

    def tx_search_rpc(self, events: str):
        rsp = requests.get(
            f"{self.node_rpc_http}/tx_search",
            params={
                "query": f'"{events}"',
            },
        ).json()
        assert "error" not in rsp, rsp["error"]
        return rsp["result"]["txs"]
