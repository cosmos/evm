import json
import subprocess
import tempfile

import requests
from pystarport.utils import build_cli_args_safe, interact, parse_amount

from .utils import (
    DEFAULT_DENOM,
    DEFAULT_GAS,
    DEFAULT_GAS_PRICE,
    MNEMONICS,
    get_sync_info,
)


class ChainCommand:
    def __init__(self, cmd):
        self.cmd = cmd

    def __call__(self, cmd, *args, stdin=None, stderr=subprocess.STDOUT, **kwargs):
        "execute evmd"
        args = " ".join(build_cli_args_safe(cmd, *args, **kwargs))
        return interact(f"{self.cmd} {args}", input=stdin, stderr=stderr)


class CosmosCLI:
    "the apis to interact with wallet and blockchain"

    def __init__(
        self,
        data_dir,
        node_rpc,
        cmd,
        chain_id=None,
    ):
        self.data_dir = data_dir
        genesis_path = self.data_dir / "config" / "genesis.json"
        self.raw = ChainCommand(cmd)
        if genesis_path.exists():
            self._genesis = json.loads(genesis_path.read_text())
            self.chain_id = self._genesis["chain_id"]
        else:
            self._genesis = {}
            self.chain_id = chain_id
            # avoid client.yml overwrite flag in textual mode
            self.raw(
                "config", "set", "client", "chain-id", chain_id, home=self.data_dir
            )
            self.raw("config", "set", "client", "node", node_rpc, home=self.data_dir)
        self.node_rpc = node_rpc
        self.output = None
        self.error = None

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

    def validators(self):
        return json.loads(
            self.raw("q", "staking", "validators", output="json", node=self.node_rpc)
        )["validators"]

    def status(self):
        return json.loads(self.raw("status", node=self.node_rpc))

    def block_height(self):
        return int(get_sync_info(self.status())["latest_block_height"])

    def balances(self, addr, height=0, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "bank",
                "balances",
                addr,
                height=height,
                **(self.get_base_kwargs() | kwargs),
            )
        )["balances"]

    def balance(self, addr, denom=DEFAULT_DENOM, height=0):
        denoms = {
            coin["denom"]: int(coin["amount"])
            for coin in self.balances(addr, height=height)
        }
        return denoms.get(denom, 0)

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

    def account(self, addr, **kwargs):
        return json.loads(
            self.raw("q", "auth", "account", addr, **(self.get_base_kwargs() | kwargs))
        )

    def transfer(
        self,
        from_,
        to,
        coins,
        generate_only=False,
        event_query_tx=True,
        fees=None,
        **kwargs,
    ):
        rsp = json.loads(
            self.raw(
                "tx",
                "bank",
                "send",
                from_,
                to,
                coins,
                "-y",
                "--generate-only" if generate_only else None,
                fees=fees,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0 and event_query_tx:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def event_query_tx_for(self, hash, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "event-query-tx-for",
                hash,
                **(self.get_base_kwargs() | kwargs),
            )
        )

    def comet_validator_set(self, height, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "comet-validator-set",
                height,
                **(self.get_base_kwargs() | kwargs),
            )
        )

    def query_all_txs(self, addr, **kwargs):
        txs = self.raw(
            "q",
            "txs-all",
            addr,
            **(self.get_base_kwargs() | kwargs),
        )
        return json.loads(txs)

    def broadcast_tx(self, tx_file, **kwargs):
        kwargs.setdefault("broadcast_mode", "sync")
        kwargs.setdefault("output", "json")
        rsp = json.loads(
            self.raw("tx", "broadcast", tx_file, node=self.node_rpc, **kwargs)
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"], **kwargs)
        return rsp

    def broadcast_tx_json(self, tx, **kwargs):
        with tempfile.NamedTemporaryFile("w") as fp:
            json.dump(tx, fp)
            fp.flush()
            return self.broadcast_tx(fp.name, **kwargs)

    def sign_tx(self, tx_file, signer, **kwargs):
        default_kwargs = self.get_kwargs()
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer,
                **(default_kwargs | kwargs),
            )
        )

    def sign_tx_json(self, tx, signer, max_priority_price=None, **kwargs):
        if max_priority_price is not None:
            tx["body"]["extension_options"].append(
                {
                    "@type": "/cosmos.evm.ante.v1.ExtensionOptionDynamicFeeTx",
                    "max_priority_price": str(max_priority_price),
                }
            )
        with tempfile.NamedTemporaryFile("w") as fp:
            json.dump(tx, fp)
            fp.flush()
            return self.sign_tx(fp.name, signer, **kwargs)

    def create_account(self, name, mnemonic=None, **kwargs):
        "create new keypair in node's keyring"
        if kwargs.get("coin_type", 60) == 60:
            kwargs.update({"coin_type": 60, "key_type": "eth_secp256k1"})
        default_kwargs = self.get_kwargs()
        args = {**default_kwargs, **kwargs}
        if mnemonic is None:
            if kwargs.get("source"):
                output = self.raw("keys", "add", name, "--recover", **args)
            else:
                output = self.raw("keys", "add", name, **args)
        else:
            output = self.raw(
                "keys",
                "add",
                name,
                "--recover",
                stdin=mnemonic.encode() + b"\n",
                **args,
            )
        return json.loads(output)

    def list_accounts(self, **kwargs):
        return json.loads(
            self.raw(
                "keys",
                "list",
                **(self.get_base_kwargs() | kwargs),
            )
        )

    def build_evm_tx(self, raw_tx: str, **kwargs):
        default_kwargs = self.get_kwargs()
        return json.loads(
            self.raw(
                "tx",
                "evm",
                "raw",
                raw_tx,
                "-y",
                "--generate-only",
                **(default_kwargs | kwargs),
            )
        )

    def submit_gov_proposal(self, proposal, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "gov",
                "submit-proposal",
                proposal,
                "-y",
                stderr=subprocess.DEVNULL,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def query_grant(self, granter, grantee, **kwargs):
        "query grant details by granter and grantee addresses"
        res = json.loads(
            self.raw(
                "q",
                "feegrant",
                "grant",
                granter,
                grantee,
                **(self.get_base_kwargs() | kwargs),
            )
        )
        res = res.get("allowance") or res
        return res

    def query_proposal(self, proposal_id, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "gov",
                "proposal",
                proposal_id,
                **(self.get_base_kwargs() | kwargs),
            )
        )
        return res.get("proposal") or res

    def query_proposals(self, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "gov",
                "proposals",
                **(self.get_base_kwargs() | kwargs),
            )
        )
        return res.get("proposals") or res

    def staking_pool(self, bonded=True, **kwargs):
        res = self.raw("q", "staking", "pool", **(self.get_base_kwargs() | kwargs))
        res = json.loads(res)
        res = res.get("pool") or res
        return int(res["bonded_tokens" if bonded else "not_bonded_tokens"])

    def delegate_amount(self, validator_address, amt, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "staking",
                "delegate",
                validator_address,
                amt,
                "--generate-only" if generate_only else None,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def unbond_amount(self, to_addr, amt, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "staking",
                "unbond",
                to_addr,
                amt,
                "-y",
                "--generate-only" if generate_only else None,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def create_validator(
        self,
        amt,
        options,
        generate_only=False,
        **kwargs,
    ):
        options = {
            "commission-max-change-rate": "0.01",
            "commission-rate": "0.1",
            "commission-max-rate": "0.2",
            "min-self-delegation": "1",
            "amount": amt,
        } | options

        if "pubkey" not in options:
            pubkey = (
                self.raw(
                    "comet",
                    "show-validator",
                    home=self.data_dir,
                )
                .strip()
                .decode()
            )
            options["pubkey"] = json.loads(pubkey)

        with tempfile.NamedTemporaryFile("w") as fp:
            json.dump(options, fp)
            fp.flush()
            raw = self.raw(
                "tx",
                "staking",
                "create-validator",
                fp.name,
                "-y",
                "--generate-only" if generate_only else None,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        rsp = json.loads(raw)
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def delegation(self, del_addr, val_addr, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "staking",
                "delegation",
                del_addr,
                val_addr,
                **(self.get_base_kwargs() | kwargs),
            )
        )
        return res.get("delegation_response") or res

    def delegations(self, del_addr, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "staking",
                "delegations",
                del_addr,
                **(self.get_base_kwargs() | kwargs),
            )
        )
        return res.get("delegation_responses") or res

    def redelegate(
        self,
        from_validator,
        to_validator,
        amt,
        generate_only=False,
        **kwargs,
    ):
        rsp = json.loads(
            self.raw(
                "tx",
                "staking",
                "redelegate",
                from_validator,
                to_validator,
                amt,
                "--generate-only" if generate_only else None,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def validator(self, addr, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "staking",
                "validator",
                addr,
                **(self.get_base_kwargs() | kwargs),
            )
        )
        return res.get("validator") or res

    def tx_simulate(self, tx, **kwargs):
        default_kwargs = self.get_kwargs()
        return json.loads(
            self.raw(
                "tx",
                "simulate",
                tx,
                **(default_kwargs | kwargs),
            )
        )

    def query_tally(self, proposal_id, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "gov",
                "tally",
                proposal_id,
                **(self.get_base_kwargs() | kwargs),
            )
        )
        return res.get("tally") or res

    def gov_vote(self, voter, proposal_id, option, event_query_tx=True, **kwargs):
        default_kwargs = self.get_kwargs()
        rsp = json.loads(
            self.raw(
                "tx",
                "gov",
                "vote",
                proposal_id,
                option,
                "-y",
                from_=voter,
                **(default_kwargs | kwargs),
            )
        )
        if rsp.get("code") == 0 and event_query_tx:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def query_bank_send(self, *denoms, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "bank",
                "send-enabled",
                *denoms,
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("send_enabled", [])

    def make_multisig(self, name, signer1, signer2, **kwargs):
        self.raw(
            "keys",
            "add",
            name,
            multisig=f"{signer1},{signer2}",
            multisig_threshold="2",
            **(self.get_kwargs() | kwargs),
        )

    def sign_multisig_tx(self, tx_file, multi_addr, signer_name, **kwargs):
        return json.loads(
            self.raw(
                "tx",
                "sign",
                tx_file,
                from_=signer_name,
                multisig=multi_addr,
                **(self.get_kwargs() | kwargs),
            )
        )

    def combine_multisig_tx(
        self, tx_file, multi_name, signer1_file, signer2_file, **kwargs
    ):
        default_kwargs = self.get_kwargs()
        return json.loads(
            self.raw(
                "tx",
                "multisign",
                tx_file,
                multi_name,
                signer1_file,
                signer2_file,
                **(default_kwargs | kwargs),
            )
        )

    def account_by_num(self, num, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "auth",
                "address-by-acc-num",
                num,
                **(self.get_base_kwargs() | kwargs),
            )
        )

    def get_base_kwargs(self):
        return {
            "home": self.data_dir,
            "node": self.node_rpc,
            "output": "json",
        }

    def get_kwargs(self):
        return self.get_base_kwargs() | {
            "keyring_backend": "test",
            "chain_id": self.chain_id,
        }

    def get_kwargs_with_gas(self):
        return self.get_kwargs() | {
            "gas_prices": DEFAULT_GAS_PRICE,
            "gas": DEFAULT_GAS,
        }

    def software_upgrade(self, proposer, proposal, **kwargs):
        default_kwargs = self.get_kwargs()
        rsp = json.loads(
            self.raw(
                "tx",
                "upgrade",
                "software-upgrade",
                proposal["name"],
                "-y",
                "--no-validate",
                from_=proposer,
                # content
                title=proposal.get("title"),
                note=proposal.get("note"),
                upgrade_height=proposal.get("upgrade-height"),
                upgrade_time=proposal.get("upgrade-time"),
                upgrade_info=proposal.get("upgrade-info"),
                summary=proposal.get("summary"),
                deposit=proposal.get("deposit"),
                # basic
                **(default_kwargs | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def get_params(self, module, **kwargs):
        default_kwargs = self.get_base_kwargs()
        return json.loads(self.raw("q", module, "params", **(default_kwargs | kwargs)))

    def query_base_fee(self, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "feemarket",
                "base-fee",
                **(self.get_base_kwargs() | kwargs),
            )
        )["base_fee"]

    def create_tokenfactory_denom(self, subdenom, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "tokenfactory",
                "create-denom",
                subdenom,
                "--generate-only" if generate_only else None,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def query_tokenfactory_denoms(self, creator, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "tokenfactory",
                "denoms-from-creator",
                creator,
                **(self.get_base_kwargs() | kwargs),
            )
        )

    def mint_tokenfactory_denom(self, coin, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "tokenfactory",
                "mint",
                coin,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def burn_tokenfactory_denom(self, coin, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "tokenfactory",
                "burn",
                coin,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def tx_search(self, events: str):
        return json.loads(
            self.raw("q", "txs", query=f'"{events}"', output="json", node=self.node_rpc)
        )

    def tx_search_rpc(self, events: str):
        rsp = requests.get(
            f"{self.node_rpc_http}/tx_search",
            params={
                "query": f'"{events}"',
            },
        ).json()
        assert "error" not in rsp, rsp["error"]
        return rsp["result"]["txs"]

    def query_erc20_token_pair(self, token, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "erc20",
                "token-pair",
                token,
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("token_pair", {})

    def query_erc20_token_pairs(self, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "erc20",
                "token-pairs",
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("token_pairs", [])

    def convert_erc20(self, contract, amt, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "erc20",
                "convert-erc20",
                contract,
                amt,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def register_erc20(self, contract, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "erc20",
                "register-erc20",
                contract,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def rollback(self):
        self.raw("rollback", home=self.data_dir)

    def prune(self, kind="everything"):
        return self.raw("prune", kind, home=self.data_dir).decode()

    def set_tokenfactory_denom(self, meta, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "tokenfactory",
                "set-denom-metadata",
                meta,
                "--generate-only" if generate_only else None,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def query_bank_denom_metadata(self, denom, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "bank",
                "denom-metadata",
                denom,
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("metadata")

    def query_denom_authority_metadata(self, denom, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "tokenfactory",
                "denom-authority-metadata",
                denom,
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("authority_metadata")

    def update_tokenfactory_admin(self, denom, address, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "tokenfactory",
                "change-admin",
                denom,
                address,
                "--generate-only" if generate_only else None,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def set_tokenfactory_before_send_hook(self, denom, address, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "tokenfactory",
                "set-before-send-hook",
                denom,
                address,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def fund_community_pool(self, amt, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "distribution",
                "fund-community-pool",
                "-y",
                amt,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def fund_validator_rewards_pool(self, val_addr, amt, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "distribution",
                "fund-validator-rewards-pool",
                "-y",
                val_addr,
                amt,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def set_withdraw_addr(self, addr, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "distribution",
                "set-withdraw-addr",
                "-y",
                addr,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def withdraw_all_rewards(self, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "distribution",
                "withdraw-all-rewards",
                "-y",
                "--generate-only" if generate_only else None,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def withdraw_rewards(self, val_addr, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "distribution",
                "withdraw-rewards",
                val_addr,
                "-y",
                "--generate-only" if generate_only else None,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def withdraw_validator_commission(self, val_addr, generate_only=False, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "distribution",
                "withdraw-validator-commission",
                val_addr,
                "-y",
                "--generate-only" if generate_only else None,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def distribution_reward(self, delegator_addr, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "distribution",
                "rewards",
                delegator_addr,
                **(self.get_base_kwargs() | kwargs),
            )
        )
        total = res.get("total")
        if not total or total[0] is None:
            return 0
        return parse_amount(total[0])

    def distribution_commission(self, addr, **kwargs):
        res = (
            json.loads(
                self.raw(
                    "q",
                    "distribution",
                    "commission",
                    addr,
                    **(self.get_base_kwargs() | kwargs),
                )
            )
            .get("commission")
            .get("commission")
        )
        if not res or not res[0]:
            return 0
        return parse_amount(res[0])

    def distribution_community_pool(self, **kwargs):
        res = json.loads(
            self.raw(
                "q",
                "distribution",
                "community-pool",
                output="json",
                node=self.node_rpc,
                **kwargs,
            )
        ).get("pool")
        return parse_amount(res[0])

    def query_disabled_list(self, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "circuit",
                "disabled-list",
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("disabled_list", [])

    def grant_authorization(self, grantee, authz_type, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "authz",
                "grant",
                grantee,
                authz_type,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def exec_tx_by_grantee(self, tx_file, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "authz",
                "exec",
                tx_file,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def revoke_authorization(self, grantee, msg_type, **kwargs):
        rsp = json.loads(
            self.raw(
                "tx",
                "authz",
                "revoke",
                grantee,
                msg_type,
                "-y",
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def query_grants(self, granter, grantee, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "authz",
                "grants",
                granter,
                grantee,
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("grants", [])

    def ibc_denom_hash(self, path, **kwargs):
        return json.loads(
            self.raw(
                "q",
                "ibc-transfer",
                "denom-hash",
                path,
                **(self.get_base_kwargs() | kwargs),
            )
        ).get("hash")

    def ibc_transfer(
        self,
        to,
        amount,
        channel,  # src channel
        generate_only=False,
        **kwargs,
    ):
        rsp = json.loads(
            self.raw(
                "tx",
                "ibc-transfer",
                "transfer",
                "transfer",
                channel,
                to,
                amount,
                "-y",
                "--generate-only" if generate_only else None,
                **(self.get_kwargs_with_gas() | kwargs),
            )
        )
        if rsp.get("code") == 0:
            rsp = self.event_query_tx_for(rsp["txhash"])
        return rsp

    def export(self, **kwargs):
        raw = self.raw("export", home=self.data_dir, **kwargs)
        if isinstance(raw, bytes):
            raw = raw.decode()
        # skip oracle client log
        idx = raw.find("{")
        if idx == -1:
            raise ValueError("No JSON object found in export output")
        return json.loads(raw[idx:])

    def has_module(self, module):
        try:
            self.raw("q", module)
            return True
        except AssertionError:
            return False
