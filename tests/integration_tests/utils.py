import base64
import binascii
import configparser
import hashlib
import json
import os
import re
import secrets
import subprocess
import sys
import time
from collections import defaultdict
from concurrent.futures import ThreadPoolExecutor, as_completed
from itertools import takewhile
from pathlib import Path

import bech32
import eth_utils
import requests
import rlp
from dateutil.parser import isoparse
from dotenv import load_dotenv
from eth_account import Account
from eth_contract.contract import Contract as ContractAsync
from eth_contract.create2 import create2_address
from eth_contract.deploy_utils import (
    ensure_create2_deployed,
    ensure_deployed_by_create2,
)
from eth_contract.erc20 import ERC20
from eth_contract.utils import ZERO_ADDRESS, balance_of, get_initcode
from eth_contract.utils import send_transaction as send_transaction_async
from eth_contract.weth import WETH, WETH9_ARTIFACT
from eth_utils import to_checksum_address
from hexbytes import HexBytes
from pystarport import cluster
from pystarport.utils import (
    wait_for_block_time,
    wait_for_new_blocks,
)
from web3 import AsyncWeb3
from web3._utils.transactions import fill_nonce, fill_transaction_defaults

load_dotenv(Path(__file__).parent.parent / "scripts/.env")
Account.enable_unaudited_hdwallet_features()
MNEMONICS = {
    "validator": os.getenv("VALIDATOR1_MNEMONIC"),
    "validator2": os.getenv("VALIDATOR2_MNEMONIC"),
    "validator3": os.getenv("VALIDATOR3_MNEMONIC"),
    "community": os.getenv("COMMUNITY_MNEMONIC"),
    "signer1": os.getenv("SIGNER1_MNEMONIC"),
    "signer2": os.getenv("SIGNER2_MNEMONIC"),
    "reserve": os.getenv("RESERVE_MNEMONIC"),
}
ACCOUNTS = {
    name: Account.from_mnemonic(mnemonic) for name, mnemonic in MNEMONICS.items()
}
KEYS = {name: account.key for name, account in ACCOUNTS.items()}
ADDRS = {name: account.address for name, account in ACCOUNTS.items()}

DEFAULT_DENOM = os.getenv("EVM_DENOM", "atest")
DEFAULT_EXTENDED_DENOM = os.getenv("EVM_EXTENDED_DENOM", "atest")
CHAIN_ID = os.getenv("CHAIN_ID", "evm-canary-net-1")
EVM_CHAIN_ID = int(os.getenv("EVM_CHAIN_ID", 7888))
# the default initial base fee used by integration tests
DEFAULT_GAS_AMT = float(os.getenv("DEFAULT_GAS_AMT", 10000000000))
DEFAULT_GAS_PRICE = f"{DEFAULT_GAS_AMT}{DEFAULT_DENOM}"
DEFAULT_GAS = 200000
DEFAULT_FEE = int(DEFAULT_GAS_AMT * DEFAULT_GAS)
WEI_PER_ETH = 10**18  # 10^18 wei == 1 ether
WEI_PER_DENOM = int(os.getenv("WEI_PER_DENOM", 1))  # 1 wei == 1 atest
ADDRESS_PREFIX = os.getenv("ADDRESS_PREFIX", "cosmos")
CMD = os.getenv("CMD", "evmd")


WETH_SALT = 999
WETH_ADDRESS = create2_address(get_initcode(WETH9_ARTIFACT), WETH_SALT)

MockERC20_ARTIFACT = json.loads(
    Path(__file__).parent.joinpath("contracts/contracts/MockERC20.json").read_text()
)


class AsyncContract:
    def __init__(self, name, key=KEYS["community"]):
        self.acct = Account.from_key(key)
        self.name = name
        self.contract = None
        self.address = None
        self.w3 = None

    async def deploy(self, w3: AsyncWeb3, args=()):
        if self.contract:
            return self.address
        self.w3 = w3
        res = build_contract(self.name)
        tx = await create_contract_transaction(w3, res, args, key=self.acct.key)
        receipt = await send_transaction_async(w3, self.acct, **tx)
        self.contract = ContractAsync(res["abi"])
        self.address = receipt.contractAddress
        return self.address

    def _check_deployed(self):
        if not self.contract:
            raise ValueError("Contract not deployed yet")


class AsyncGreeter(AsyncContract):
    def __init__(self, key=KEYS["community"]):
        super().__init__("Greeter", key)

    async def greet(self):
        self._check_deployed()
        return await self.contract.fns.greet().call(self.w3, to=self.address)

    async def int_value(self):
        self._check_deployed()
        return await self.contract.fns.intValue().call(self.w3, to=self.address)

    async def set_greeting(self, message: str):
        self._check_deployed()
        return await self.contract.fns.setGreeting(message).transact(
            self.w3, self.acct, to=self.address
        )


class AsyncTestRevert(AsyncContract):
    def __init__(self, key=KEYS["community"]):
        super().__init__("TestRevert", key)

    async def transfer(self, value):
        self._check_deployed()
        return await self.contract.fns.transfer(value).transact(
            self.w3,
            self.acct,
            to=self.address,
            gas=100000,  # skip estimateGas error
        )


class AsyncTestMessageCall(AsyncContract):
    def __init__(self, key=KEYS["community"]):
        super().__init__("TestMessageCall", key)

    async def test(self, iterations):
        self._check_deployed()
        return await self.contract.fns.test(iterations).transact(
            self.w3, self.acct, to=self.address
        )

    def get_test_data(self, iterations):
        self._check_deployed()
        return self.contract.fns.test(iterations).data


class Contract:
    def __init__(self, name, private_key=KEYS["community"], chain_id=EVM_CHAIN_ID):
        self.chain_id = chain_id
        self.account = Account.from_key(private_key)
        self.owner = self.account.address
        self.private_key = private_key
        res = build_contract(name)
        self.bytecode = res["bytecode"]
        self.code = res["code"]
        self.abi = res["abi"]
        self.contract = None
        self.w3 = None

    def deploy(self, w3, exp_gas_used=None):
        "Deploy contract on `w3` and return the receipt."
        if self.contract is None:
            self.w3 = w3
            contract = self.w3.eth.contract(abi=self.abi, bytecode=self.bytecode)
            transaction = contract.constructor().build_transaction(
                {"chainId": self.chain_id, "from": self.owner}
            )
            receipt = send_transaction(self.w3, transaction, self.private_key)
            if exp_gas_used is not None:
                assert (
                    exp_gas_used == receipt.gasUsed
                ), f"exp {exp_gas_used}, got {receipt.gasUsed}"
            self.contract = self.w3.eth.contract(
                address=receipt.contractAddress, abi=self.abi
            )
            return receipt
        else:
            return receipt


class Greeter(Contract):
    "Greeter contract."

    def transfer(self, string):
        "Call contract on `w3` and return the receipt."
        transaction = self.contract.functions.setGreeting(string).build_transaction(
            {
                "chainId": self.chain_id,
                "from": self.owner,
            }
        )
        receipt = send_transaction(self.w3, transaction, self.private_key)
        assert string == self.contract.functions.greet().call()
        return receipt


class RevertTestContract(Contract):
    "RevertTestContract contract."

    def transfer(self, value):
        "Call contract on `w3` and return the receipt."
        transaction = self.contract.functions.transfer(value).build_transaction(
            {
                "chainId": self.chain_id,
                "from": self.owner,
                "gas": 100000,  # skip estimateGas error
            }
        )
        receipt = send_transaction(self.w3, transaction, self.private_key)
        return receipt


def supervisorctl(inipath, *args):
    return subprocess.check_output(
        (sys.executable, "-msupervisor.supervisorctl", "-c", inipath, *args),
    ).decode()


def find_log_event_attrs(events, ev_type, cond=None):
    for ev in events:
        if ev["type"] == ev_type:
            attrs = {attr["key"]: attr["value"] for attr in ev["attributes"]}
            if cond is None or cond(attrs):
                return attrs
    return None


def find_duplicate(attributes):
    res = set()
    key = attributes[0]["key"]
    for attribute in attributes:
        if attribute["key"] == key:
            value0 = attribute["value"]
        elif attribute["key"] == "amount":
            amount = attribute["value"]
            value_pair = f"{value0}:{amount}"
            if value_pair in res:
                return value_pair
            res.add(value_pair)
    return None


def sign_transaction(w3, tx, key=KEYS["community"]):
    "fill default fields and sign"
    acct = Account.from_key(key)
    tx["from"] = acct.address
    tx = fill_transaction_defaults(w3, tx)
    tx = fill_nonce(w3, tx)
    return acct.sign_transaction(tx)


def send_raw_transactions(w3, raw_transactions):
    with ThreadPoolExecutor(len(raw_transactions)) as exec:
        tasks = [
            exec.submit(w3.eth.send_raw_transaction, raw) for raw in raw_transactions
        ]
        sended_hash_set = {future.result() for future in as_completed(tasks)}
    return sended_hash_set


def send_transaction(w3, tx, key=KEYS["community"], check=True):
    signed = sign_transaction(w3, tx, key)
    txhash = w3.eth.send_raw_transaction(signed.raw_transaction)
    if check:
        return w3.eth.wait_for_transaction_receipt(txhash)
    return txhash


def send_txs(w3, cli, to, keys, params):
    tx = {"to": to, "value": 10000} | params
    # use different sender accounts to be able be send concurrently
    raw_transactions = []
    for key_from in keys:
        signed = sign_transaction(w3, tx, key_from)
        raw_transactions.append(signed.raw_transaction)

    # wait block update
    block_num_0 = wait_for_new_blocks(cli, 1, sleep=0.1)
    print(f"block number start: {block_num_0}")

    # send transactions
    sended_hash_set = send_raw_transactions(w3, raw_transactions)
    return block_num_0, sended_hash_set


# Global cache for built contracts
CONTRACTS = {}


def build_contract(name, dir="contracts") -> dict:
    if name in CONTRACTS:
        return CONTRACTS[name]
    cmd = [
        "solc",
        "--abi",
        "--bin",
        "--bin-runtime",
        f"contracts/{dir}/{name}.sol",
        "-o",
        "build",
        "--overwrite",
        "--optimize",
        "--optimize-runs",
        "100000",
        "--via-ir",
        "--metadata-hash",
        "none",
        "--no-cbor-metadata",
        "--base-path",
        "./contracts",
        "--allow-paths",
        "../../precompiles",
        # "$(cat contracts/remappings.txt)",
    ]
    with open("contracts/remappings.txt", "r") as f:
        remappings = f.read().strip().split()

    cmd.extend(remappings)
    print(*cmd)
    subprocess.run(cmd, check=True)
    bytecode = Path(f"build/{name}.bin").read_text().strip()
    code = Path(f"build/{name}.bin-runtime").read_text().strip()
    result = {
        "abi": json.loads(Path(f"build/{name}.abi").read_text()),
        "bytecode": f"0x{bytecode}",
        "code": f"0x{code}",
    }
    CONTRACTS[name] = result
    return result


async def build_and_deploy_contract_async(
    w3: AsyncWeb3,
    name,
    args=(),
    key=KEYS["community"],
    dir="contracts",
):
    res = build_contract(name, dir=dir)
    tx = await create_contract_transaction(w3, res, args, key, dir=dir)
    txreceipt = await send_transaction_async(w3, Account.from_key(key), **tx)
    return w3.eth.contract(address=txreceipt.contractAddress, abi=res["abi"])


def create_contract_transaction(
    w3, name_or_res, args=(), key=KEYS["community"], dir="contracts"
):
    acct = Account.from_key(key)
    if isinstance(name_or_res, str):
        res = build_contract(name_or_res, dir=dir)
    else:
        res = name_or_res
    contract = w3.eth.contract(abi=res["abi"], bytecode=res["bytecode"])
    return contract.constructor(*args).build_transaction({"from": acct.address})


def eth_to_bech32(addr, prefix=ADDRESS_PREFIX):
    bz = bech32.convertbits(HexBytes(addr), 8, 5)
    return bech32.bech32_encode(prefix, bz)


def decode_bech32(addr):
    _, bz = bech32.bech32_decode(addr)
    return HexBytes(bytes(bech32.convertbits(bz, 5, 8)))


def bech32_to_eth(addr):
    return to_checksum_address(decode_bech32(addr).hex())


def hash_func(address_type_bytes, key):
    hasher = hashlib.sha256()
    hasher.update(address_type_bytes)
    th = hasher.digest()
    hasher = hashlib.sha256()
    hasher.update(th)
    hasher.update(key)
    return hasher.digest()


def derive(address_type_bytes, key):
    return hash_func(address_type_bytes, key)


def module_address(name, *derivation_keys):
    m_key = name.encode()
    if len(derivation_keys) == 0:
        address_bytes = hashlib.sha256(m_key).digest()[:20]
    else:
        m_key = m_key + b"\x00"
        first_key = m_key + derivation_keys[0]
        addr = hash_func("module".encode(), first_key)
        for k in derivation_keys[1:]:
            addr = derive(addr, k)
        address_bytes = addr[:20]
    eth_address = "0x" + address_bytes.hex()
    return eth_to_bech32(eth_address)


def generate_isolated_address(channel_id, sender):
    name = "ibc-callbacks"
    return module_address(name, channel_id.encode(), sender.encode())


def get_balance(cli, name):
    try:
        addr = cli.address(name, skip_create=True)
    except Exception as e:
        if "key not found" not in str(e):
            raise
        addr = name
    atest = cli.balance(addr)
    return atest


def assert_balance(cli, w3, name, evm=False):
    try:
        addr = cli.address(name, skip_create=True)
    except Exception as e:
        if "key not found" not in str(e):
            raise
        addr = name
    atest = get_balance(cli, name)
    wei = w3.eth.get_balance(bech32_to_eth(addr))
    assert atest == wei // WEI_PER_DENOM
    print(
        f"wei: {wei}, ether: {wei // WEI_PER_ETH}.",
    )
    return wei if evm else atest


def find_fee(rsp):
    res = find_log_event_attrs(rsp["events"], "tx", lambda attrs: "fee" in attrs)
    return int("".join(takewhile(lambda s: s.isdigit() or s == ".", res["fee"])))


def assert_transfer(cli, addr_a, addr_b, amt=1):
    balance_a = cli.balance(addr_a)
    balance_b = cli.balance(addr_b)
    rsp = cli.transfer(addr_a, addr_b, f"{amt}{DEFAULT_DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]
    fee = find_fee(rsp)
    assert cli.balance(addr_a) == balance_a - amt - fee
    assert cli.balance(addr_b) == balance_b + amt


def denom_to_erc20_address(denom):
    denom_hash = hashlib.sha256(denom.encode()).digest()
    return to_checksum_address("0x" + denom_hash[-20:].hex())


def escrow_address(port, channel, prefix=ADDRESS_PREFIX):
    escrow_addr_version = "ics20-1"
    pre_image = f"{escrow_addr_version}\x00{port}/{channel}"
    return eth_to_bech32(
        hashlib.sha256(pre_image.encode()).digest()[:20].hex(), prefix=prefix
    )


def ibc_denom_address(denom):
    if not denom.startswith("ibc/"):
        raise ValueError(f"coin {denom} does not have 'ibc/' prefix")
    if len(denom) < 5 or denom[4:].strip() == "":
        raise ValueError(f"coin {denom} is not a valid IBC voucher hash")
    hash_part = denom[4:]  # remove "ibc/" prefix
    hash_bytes = binascii.unhexlify(hash_part)
    return to_checksum_address("0x" + hash_bytes[-20:].hex())


def retry_on_seq_mismatch(fn, *args, max_retries=3, **kwargs):
    for attempt in range(max_retries):
        rsp = fn(*args, **kwargs)
        if rsp["code"] == 0:
            return rsp
        if rsp["code"] == 32 and "account sequence mismatch" in rsp["raw_log"]:
            if attempt < max_retries - 1:
                continue
        return rsp
    return rsp


def recover_community(cli, tmp_path):
    return cli.create_account(
        "community",
        mnemonic=os.getenv("COMMUNITY_MNEMONIC"),
        home=tmp_path,
    )["address"]


def transfer_via_cosmos(cli, from_addr, to_addr, amount):
    tx = cli.transfer(
        from_addr,
        to_addr,
        f"{amount}{DEFAULT_DENOM}",
        generate_only=True,
        chain_id=cli.chain_id,
    )
    tx_json = cli.sign_tx_json(
        tx, from_addr, home=cli.data_dir, node=cli.node_rpc, chain_id=cli.chain_id
    )
    rsp = cli.broadcast_tx_json(tx_json, home=cli.data_dir)
    assert rsp["code"] == 0, rsp["raw_log"]
    attrs = find_log_event_attrs(rsp["events"], "tx", lambda attrs: "fee" in attrs)
    return int("".join(takewhile(lambda s: s.isdigit() or s == ".", attrs["fee"])))


class ContractAddress(rlp.Serializable):
    fields = [
        ("from", rlp.sedes.Binary()),
        ("nonce", rlp.sedes.big_endian_int),
    ]


def contract_address(addr, nonce):
    return eth_utils.to_checksum_address(
        eth_utils.to_hex(
            eth_utils.keccak(
                rlp.encode(ContractAddress(eth_utils.to_bytes(hexstr=addr), nonce))
            )[12:]
        )
    )


def build_batch_tx(w3, cli, txs, key=KEYS["community"]):
    "return cosmos batch tx and eth tx hashes"
    signed_txs = [sign_transaction(w3, tx, key) for tx in txs]
    tmp_txs = [
        cli.build_evm_tx(f"0x{s.raw_transaction.hex()}", chain_id=EVM_CHAIN_ID)
        for s in signed_txs
    ]

    msgs = [tx["body"]["messages"][0] for tx in tmp_txs]
    fee = sum(int(tx["auth_info"]["fee"]["amount"][0]["amount"]) for tx in tmp_txs)
    gas_limit = sum(int(tx["auth_info"]["fee"]["gas_limit"]) for tx in tmp_txs)

    tx_hashes = [signed.hash for signed in signed_txs]

    # build batch cosmos tx
    return {
        "body": {
            "messages": msgs,
            "memo": "",
            "timeout_height": "0",
            "extension_options": [
                {"@type": "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx"}
            ],
            "non_critical_extension_options": [],
        },
        "auth_info": {
            "signer_infos": [],
            "fee": {
                "amount": [{"denom": "atest", "amount": str(fee)}],
                "gas_limit": str(gas_limit),
                "payer": "",
                "granter": "",
            },
        },
        "signatures": [],
    }, tx_hashes


def approve_proposal(n, events, event_query_tx=True, **kwargs):
    cli = n.cosmos_cli()
    # get proposal_id
    ev = find_log_event_attrs(
        events, "submit_proposal", lambda attrs: "proposal_id" in attrs
    )
    proposal_id = ev["proposal_id"]
    for i in range(len(n.config["validators"])):
        rsp = n.cosmos_cli(i).gov_vote(
            "validator",
            proposal_id,
            "yes",
            event_query_tx=event_query_tx,
            **kwargs,
        )
        assert rsp["code"] == 0, rsp["raw_log"]
    wait_for_new_blocks(cli, 1)
    res = cli.query_tally(proposal_id)
    res = res.get("tally") or res
    assert (
        int(res["yes_count"]) == cli.staking_pool()
    ), "all validators should have voted yes"
    print("wait for proposal to be activated")
    proposal = cli.query_proposal(proposal_id)
    wait_for_block_time(cli, isoparse(proposal["voting_end_time"]))
    proposal = cli.query_proposal(proposal_id)
    assert proposal["status"] == "PROPOSAL_STATUS_PASSED", proposal


def submit_gov_proposal(evm, tmp_path, messages, event_query_tx=True, **kwargs):
    proposal = tmp_path / "proposal.json"
    proposal_src = {
        "title": "title",
        "summary": "summary",
        "deposit": f"1{DEFAULT_DENOM}",
        "messages": messages,
    }
    proposal.write_text(json.dumps(proposal_src))
    rsp = evm.cosmos_cli().submit_gov_proposal(proposal, from_="community", **kwargs)
    assert rsp["code"] == 0, rsp["raw_log"]
    approve_proposal(evm, rsp["events"], event_query_tx=event_query_tx)
    print("check params have been updated now")
    return rsp


def create_periodic_vesting_acct(cli, tmp_path, coin, **kwargs):
    start_time = int(time.time())
    periods = tmp_path / "periods.json"
    src = {
        "start_time": start_time,
        "periods": [{"coins": coin, "length_seconds": 2592000}],
    }
    periods.write_text(json.dumps(src))
    name = f"periodic_vesting{start_time}"
    addr = cli.create_account(name)["address"]
    rsp = cli.create_periodic_vesting_account(addr, periods, **kwargs)
    assert rsp["code"] == 0, rsp["raw_log"]
    return addr


def derive_new_account(n=1, mnemonic="SIGNER1_MNEMONIC"):
    # derive a new address
    account_path = f"m/44'/60'/0'/0/{n}"
    return Account.from_mnemonic(os.getenv(mnemonic), account_path=account_path)


def derive_random_account():
    return derive_new_account(secrets.randbelow(10000) + 1)


def edit_ini_sections(chain_id, ini_path, callback):
    ini = configparser.RawConfigParser()
    ini.read(ini_path)
    reg = re.compile(rf"^program:{chain_id}-node(\d+)")
    for section in ini.sections():
        m = reg.match(section)
        if m:
            i = m.group(1)
            old = ini[section]
            ini[section].update(callback(i, old))
    with ini_path.open("w") as fp:
        ini.write(fp)


def adjust_base_fee(parent_fee, gas_limit, gas_used, params):
    "spec: https://eips.ethereum.org/EIPS/eip-1559#specification"
    params = {k: float(v) for k, v in params.items()}
    change_denominator = params.get("base_fee_change_denominator", 8)
    elasticity_multiplier = params.get("elasticity_multiplier", 2)
    gas_target = gas_limit // elasticity_multiplier
    if gas_used == gas_target:
        return parent_fee
    delta = parent_fee * abs(gas_target - gas_used) // gas_target // change_denominator
    # https://github.com/cosmos/evm/blob/0e511d32206b1ac709a0eb0ddb1aa21d29e833b8/x/feemarket/keeper/eip1559.go#L93
    if gas_target > gas_used:
        min_gas_price = float(params.get("min_gas_price", 0)) * WEI_PER_DENOM
        return max(parent_fee - delta, min_gas_price)
    else:
        return parent_fee + max(delta, 1)


def assert_duplicate(rpc, height):
    res = requests.get(f"{rpc}/block_results?height={height}").json().get("result")
    res = next((tx for tx in res.get("txs_results") if tx["code"] == 0), None)
    values = set()
    for event in res.get("events", []):
        if event["type"] != "transfer":
            continue
        str = json.dumps(event)
        assert str not in values, f"dup event find: {str}"
        values.add(str)


def fund_acc(w3, acc, fund=4_000_000_000_000_000_000):
    addr = acc.address
    if w3.eth.get_balance(addr, "latest") == 0:
        tx = {"to": addr, "value": fund, "gasPrice": w3.eth.gas_price}
        send_transaction(w3, tx)
        assert w3.eth.get_balance(addr, "latest") == fund


def do_multisig(cli, tmp_path, signer1_name, signer2_name, multisig_name):
    # prepare multisig and accounts
    signer1 = cli.address(signer1_name)
    signer2 = cli.address(signer2_name)
    cli.make_multisig(multisig_name, signer1_name, signer2_name)
    multi_addr = cli.address(multisig_name)
    amt = 4_000_000_000_000_000 // WEI_PER_DENOM
    rsp = cli.transfer(signer1, multi_addr, f"{amt}{DEFAULT_DENOM}")
    assert rsp["code"] == 0, rsp["raw_log"]
    acc = cli.account(multi_addr)
    res = cli.account_by_num(acc["account"]["value"]["account_number"])
    assert res["account_address"] == multi_addr

    m_txt = tmp_path / "m.txt"
    p1_txt = tmp_path / "p1.txt"
    p2_txt = tmp_path / "p2.txt"
    tx_txt = tmp_path / "tx.txt"
    amt = 1
    multi_tx = cli.transfer(
        multi_addr,
        signer2,
        f"{amt}{DEFAULT_DENOM}",
        generate_only=True,
    )
    json.dump(multi_tx, m_txt.open("w"))
    signature1 = cli.sign_multisig_tx(m_txt, multi_addr, signer1_name)
    json.dump(signature1, p1_txt.open("w"))
    signature2 = cli.sign_multisig_tx(m_txt, multi_addr, signer2_name)
    json.dump(signature2, p2_txt.open("w"))
    final_multi_tx = cli.combine_multisig_tx(
        m_txt,
        multisig_name,
        p1_txt,
        p2_txt,
    )
    json.dump(final_multi_tx, tx_txt.open("w"))
    rsp = cli.broadcast_tx(tx_txt)
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cli.account(multi_addr)["account"]["value"]["address"] == multi_addr


def decode_base64(raw):
    try:
        return base64.b64decode(raw.encode()).decode()
    except Exception:
        return raw


def parse_events_rpc(events):
    result = defaultdict(dict)
    for ev in events:
        for attr in ev["attributes"]:
            if attr["key"] is None:
                continue
            key = decode_base64(attr["key"])
            if attr["value"] is not None:
                value = decode_base64(attr["value"])
            else:
                value = None
            result[ev["type"]][key] = value
    return result


async def assert_create_erc20_denom(w3, signer):
    await ensure_create2_deployed(w3, signer)
    await ensure_deployed_by_create2(
        w3, signer, get_initcode(WETH9_ARTIFACT), salt=WETH_SALT
    )
    assert (await ERC20.fns.decimals().call(w3, to=WETH_ADDRESS)) == 18
    total_bf = await ERC20.fns.totalSupply().call(w3, to=WETH_ADDRESS)
    balance_bf = await ERC20.fns.balanceOf(signer).call(w3, to=WETH_ADDRESS)
    assert total_bf == balance_bf

    weth = WETH(to=WETH_ADDRESS)
    erc20_denom = f"erc20:{WETH_ADDRESS}"
    deposit_amt = 100
    res = await weth.fns.deposit().transact(w3, signer, value=deposit_amt)
    assert res.status == 1
    total = await ERC20.fns.totalSupply().call(w3, to=WETH_ADDRESS)
    balance = await ERC20.fns.balanceOf(signer).call(w3, to=WETH_ADDRESS)
    assert total == balance
    assert (total - total_bf) == (balance - balance_bf) == deposit_amt
    return erc20_denom, total


async def assert_weth_flow(w3, weth_addr, owner, account):
    weth = WETH(to=weth_addr)
    before = await balance_of(w3, ZERO_ADDRESS, owner)
    receipt = await weth.fns.deposit().transact(w3, account, value=1000)
    fee = receipt["effectiveGasPrice"] * receipt["gasUsed"]
    await balance_of(w3, weth_addr, owner) == 1000
    receipt = await weth.fns.withdraw(1000).transact(w3, account)
    fee += receipt["effectiveGasPrice"] * receipt["gasUsed"]
    await balance_of(w3, weth_addr, owner) == 0
    assert await balance_of(w3, ZERO_ADDRESS, owner) == before - fee
    assert await weth.fns.decimals().call(w3) == 18
    assert await weth.fns.symbol().call(w3) == "WETH"
    assert await weth.fns.name().call(w3) == "Wrapped Ether"


def address_to_bytes32(addr) -> HexBytes:
    return HexBytes(addr).rjust(32, b"\x00")


def edit_app_cfg(cli, i):
    # Modify the json-rpc addresses to avoid conflict
    cluster.edit_app_cfg(
        cli.home(i) / "config/app.toml",
        cli.base_port(i),
        {
            "json-rpc": {
                "enable": True,
                "address": "127.0.0.1:{EVMRPC_PORT}",
                "ws-address": "127.0.0.1:{EVMRPC_PORT_WS}",
            },
        },
    )


def duration(duration_str):
    mult = {"s": 1, "m": 60, "h": 3600, "d": 86400}
    parts = re.findall(r"(\d+)([smhd])", duration_str.lower())
    return sum(int(value) * mult[unit] for value, unit in parts)
