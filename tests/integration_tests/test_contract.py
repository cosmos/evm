import asyncio
import json
from pathlib import Path

import pytest
from eth_abi import encode
from eth_contract.contract import Contract, ContractFunction
from eth_contract.create2 import create2_address
from eth_contract.deploy_utils import (
    ensure_create2_deployed,
    ensure_createx_deployed,
    ensure_deployed_by_create2,
    ensure_deployed_by_create3,
    ensure_history_storage_deployed,
    ensure_multicall3_deployed,
)
from eth_contract.entrypoint import (
    ENTRYPOINT07_ADDRESS,
    ENTRYPOINT07_ARTIFACT,
    ENTRYPOINT07_SALT,
    ENTRYPOINT08_ADDRESS,
    ENTRYPOINT08_ARTIFACT,
    ENTRYPOINT08_SALT,
)
from eth_contract.erc20 import ERC20
from eth_contract.history_storage import HISTORY_STORAGE_ADDRESS
from eth_contract.multicall3 import (
    MULTICALL3,
    MULTICALL3_ADDRESS,
    Call3Value,
    multicall,
)
from eth_contract.utils import ZERO_ADDRESS, balance_of, get_initcode, send_transaction
from eth_contract.weth import WETH, WETH9_ARTIFACT
from eth_hash.auto import keccak
from eth_utils import to_bytes
from pystarport.utils import w3_wait_for_new_blocks_async
from web3 import AsyncWeb3
from web3._utils.contracts import encode_transaction_data
from web3.types import TxParams

from .utils import (
    ACCOUNTS,
    ADDRS,
    KEYS,
    WETH_ADDRESS,
    WETH_SALT,
    MockERC20_ARTIFACT,
    address_to_bytes32,
    assert_weth_flow,
    build_and_deploy_contract_async,
    build_contract,
    create_contract_transaction,
)

pytestmark = pytest.mark.asyncio


MULTICALL3ROUTER_ARTIFACT = json.loads(
    Path(__file__)
    .parent.joinpath("contracts/contracts/Multicall3Router.json")
    .read_text()
)
MULTICALL3ROUTER = create2_address(
    get_initcode(MULTICALL3ROUTER_ARTIFACT, MULTICALL3_ADDRESS)
)


async def assert_contract_deployed(w3):
    account = ACCOUNTS["community"]
    await ensure_create2_deployed(w3, account)
    await ensure_multicall3_deployed(w3, account)
    await ensure_deployed_by_create2(
        w3,
        account,
        get_initcode(WETH9_ARTIFACT),
        salt=WETH_SALT,
    )
    assert MULTICALL3ROUTER == await ensure_deployed_by_create2(
        w3,
        account,
        get_initcode(MULTICALL3ROUTER_ARTIFACT, MULTICALL3_ADDRESS),
    )
    assert await w3.eth.get_code(WETH_ADDRESS)
    assert await w3.eth.get_code(MULTICALL3ROUTER)
    assert await w3.eth.get_code(MULTICALL3_ADDRESS)


async def test_flow(evm):
    w3 = evm.async_w3
    await assert_contract_deployed(w3)
    account = ACCOUNTS["community"]
    await ensure_createx_deployed(w3, account)
    await ensure_history_storage_deployed(w3, account)
    assert await w3.eth.get_code(HISTORY_STORAGE_ADDRESS)
    salt = 100
    initcode = to_bytes(hexstr=build_contract("TestBlockTxProperties")["bytecode"][2:])
    contract = await ensure_deployed_by_create2(w3, account, initcode, salt=salt)
    assert contract == "0xe1B18c74a33b1E67B5f505C931Ac264668EA94F5"
    height = await w3.eth.block_number
    await w3_wait_for_new_blocks_async(w3, 1)

    blockhash = ContractFunction.from_abi(
        "function getBlockHash(uint256) external returns (bytes32)"
    )
    res = (await blockhash(height).call(w3, to=contract)).hex()
    blk = await w3.eth.get_block(height)
    assert res == blk.hash.hex(), res

    owner = account.address
    # test_create2_deploy
    initcode = get_initcode(MockERC20_ARTIFACT, "TEST", "TEST", 18)
    token = await ensure_deployed_by_create2(w3, account, initcode, salt=salt)
    assert token == "0x854d811d90C6E81B84b29C1d7ed957843cF87bba"
    balance = await ERC20.fns.balanceOf(owner).call(w3, to=token)
    amt = 1000
    await ERC20.fns.mint(owner, amt).transact(w3, account, to=token)
    assert await ERC20.fns.balanceOf(owner).call(w3, to=token) == balance + amt

    # test_create3_deploy
    salt = 200
    token = await ensure_deployed_by_create3(w3, account, initcode, salt=salt)
    assert token == "0x60f7B32B5799838a480572Aee2A8F0355f607b38"
    balance = await ERC20.fns.balanceOf(owner).call(w3, to=token)
    await ERC20.fns.mint(owner, 1000).transact(w3, account, to=token)
    assert await ERC20.fns.balanceOf(owner).call(w3, to=token) == balance + amt

    # test_weth
    weth = WETH(to=WETH_ADDRESS)
    await assert_weth_flow(w3, WETH_ADDRESS, owner, account)

    # test_batch_call
    users = [ACCOUNTS[key] for key in ["community", "signer1", "signer2"]]
    amount = 1000
    amount_all = amount * len(users)

    balances = [(WETH_ADDRESS, ERC20.fns.balanceOf(user.address)) for user in users]
    balances_bf = await multicall(w3, balances)
    await MULTICALL3.fns.aggregate3Value(
        [Call3Value(WETH_ADDRESS, False, amount_all, weth.fns.deposit().data)]
        + [
            Call3Value(
                WETH_ADDRESS, False, 0, ERC20.fns.transfer(user.address, amount).data
            )
            for user in users
        ]
    ).transact(w3, users[0], value=amount_all)
    balances_af = await multicall(w3, balances)
    assert all(af - bf == amount for af, bf in zip(balances_af, balances_bf))

    for user in users:
        await ERC20.fns.approve(MULTICALL3_ADDRESS, amount).transact(
            w3, user, to=WETH_ADDRESS
        )

    await MULTICALL3.fns.aggregate3Value(
        [
            Call3Value(
                WETH_ADDRESS,
                data=ERC20.fns.transferFrom(
                    user.address, MULTICALL3_ADDRESS, amount
                ).data,
            )
            for user in users
        ]
        + [
            Call3Value(
                WETH_ADDRESS,
                data=weth.fns.transferFrom(
                    MULTICALL3_ADDRESS, users[0].address, amount_all
                ).data,
            ),
        ]
    ).transact(w3, users[0])
    await weth.fns.withdraw(amount_all).transact(w3, users[0], to=WETH_ADDRESS)
    balances_bf = await multicall(w3, balances)
    await balance_of(w3, WETH_ADDRESS, MULTICALL3_ADDRESS) == 0

    # test_multicall3_router
    amount_all = amount * len(users)
    router = Contract(MULTICALL3ROUTER_ARTIFACT["abi"])
    multicall3 = MULTICALL3ROUTER

    balances = [(WETH_ADDRESS, ERC20.fns.balanceOf(user.address)) for user in users]
    balances_bf = await multicall(w3, balances)
    assert (await multicall(w3, balances)) == balances_bf
    before = await balance_of(w3, ZERO_ADDRESS, users[0].address)

    # convert amount_all into WETH and distribute to users
    receipt = await MULTICALL3.fns.aggregate3Value(
        [Call3Value(WETH_ADDRESS, False, amount_all, WETH.fns.deposit().data)]
        + [
            Call3Value(
                WETH_ADDRESS, False, 0, ERC20.fns.transfer(user.address, amount).data
            )
            for user in users
        ]
    ).transact(w3, users[0], to=multicall3, value=amount_all)
    before -= receipt["effectiveGasPrice"] * receipt["gasUsed"]
    # check users's weth balances
    balances_af = await multicall(w3, balances)
    assert all(af - bf == amount for af, bf in zip(balances_af, balances_bf))
    balances_bf = balances_af

    # approve multicall3 to transfer WETH on behalf of users
    for i, user in enumerate(users):
        receipt = await ERC20.fns.approve(multicall3, amount).transact(
            w3, user, to=WETH_ADDRESS
        )
        if i == 0:
            before -= receipt["effectiveGasPrice"] * receipt["gasUsed"]

    # transfer WETH from all users to multicall3, withdraw it,
    # and send back to users[0]
    receipt = await MULTICALL3.fns.aggregate3Value(
        [
            Call3Value(
                WETH_ADDRESS,
                data=ERC20.fns.transferFrom(user.address, multicall3, amount).data,
            )
            for user in users
        ]
        + [
            Call3Value(WETH_ADDRESS, data=WETH.fns.withdraw(amount_all).data),
            Call3Value(
                multicall3,
                data=router.fns.sellToPool(
                    ZERO_ADDRESS, 10000, users[0].address, 0, b""
                ).data,
            ),
        ]
    ).transact(w3, users[0], to=multicall3)
    before -= receipt["effectiveGasPrice"] * receipt["gasUsed"]
    balances_af = await multicall(w3, balances)
    assert all(af + amount == bf for af, bf in zip(balances_af, balances_bf))
    assert await balance_of(w3, WETH_ADDRESS, multicall3) == 0
    assert await balance_of(w3, ZERO_ADDRESS, multicall3) == 0

    # user get all funds back other than gas fees
    assert await balance_of(w3, ZERO_ADDRESS, users[0].address) == before


async def test_7702(evm):
    w3: AsyncWeb3 = evm.async_w3
    await assert_contract_deployed(w3)

    acct = ACCOUNTS["signer2"]
    sponsor = ACCOUNTS["community"]
    multicall3 = MULTICALL3ROUTER

    nonce = await w3.eth.get_transaction_count(acct.address)
    chain_id = await w3.eth.chain_id
    auth = acct.sign_authorization(
        {"chainId": chain_id, "address": multicall3, "nonce": nonce}
    )
    amount = 1000
    calls = [
        Call3Value(WETH_ADDRESS, False, amount, WETH.fns.deposit().data),
        Call3Value(WETH_ADDRESS, False, 0, WETH.fns.withdraw(amount).data),
    ]
    tx: TxParams = {
        "chainId": chain_id,
        "to": acct.address,
        "value": amount,
        "authorizationList": [auth],
        "data": MULTICALL3.fns.aggregate3Value(calls).data,
    }

    before = await w3.eth.get_balance(acct.address)
    receipt = await send_transaction(w3, sponsor, **tx)
    after = await w3.eth.get_balance(acct.address)
    assert before + amount == after

    assert await w3.eth.get_transaction_count(acct.address) == nonce + 1

    logs = receipt["logs"]
    assert logs[0]["topics"] == [
        WETH.events.Deposit.topic,
        address_to_bytes32(acct.address),
    ]
    assert logs[1]["topics"] == [
        WETH.events.Withdrawal.topic,
        address_to_bytes32(acct.address),
    ]

    assert await w3.eth.get_code(acct.address)
    block = await w3.eth.get_block(receipt["blockNumber"], True)
    assert block["transactions"][0] == await w3.eth.get_transaction(
        receipt["transactionHash"]
    )
    assert block["hash"] == block["transactions"][0]["blockHash"]
    receipts = await w3.eth.get_block_receipts(receipt["blockNumber"])
    assert receipts[0] == receipt


async def test_4337(evm):
    w3: AsyncWeb3 = evm.async_w3
    await assert_contract_deployed(w3)
    account = ACCOUNTS["community"]
    assert ENTRYPOINT08_ADDRESS == await ensure_deployed_by_create2(
        w3, account, get_initcode(ENTRYPOINT08_ARTIFACT), ENTRYPOINT08_SALT
    )
    assert ENTRYPOINT07_ADDRESS == await ensure_deployed_by_create2(
        w3, account, get_initcode(ENTRYPOINT07_ARTIFACT), ENTRYPOINT07_SALT
    )


async def test_deploy_multi(evm):
    w3 = evm.async_w3
    name = "community"
    key = KEYS[name]
    owner = ADDRS[name]
    num = 10
    res = build_contract("ERC20MinterBurnerDecimals")
    args_list = [(w3, res, (f"MyToken{i}", f"MTK{i}", 18), key) for i in range(num)]
    tx_results = await asyncio.gather(
        *(create_contract_transaction(*args) for args in args_list)
    )
    nonce = await w3.eth.get_transaction_count(owner)
    txs = [{**tx, "nonce": nonce + i} for i, tx in enumerate(tx_results)]
    receipts = await asyncio.gather(
        *(send_transaction(w3, tx["from"], **tx) for tx in txs), return_exceptions=True
    )
    for r in receipts:
        if isinstance(r, Exception):
            pytest.fail(f"send_transaction failed: {r}")
    assert len(receipts) == num
    total = 100
    token = receipts[0]["contractAddress"]
    receipt = await ERC20.fns.mint(owner, total).transact(w3, owner, to=token)
    assert receipt.status == 1
    assert await ERC20.fns.balanceOf(owner).call(w3, to=token) == total


async def test_upgrade(evm):
    w3 = evm.async_w3
    owner = ADDRS["community"]
    token = await build_and_deploy_contract_async(w3, "MyToken")
    proxy = await build_and_deploy_contract_async(
        w3,
        "ERC1967Proxy",
        args=(
            token.address,
            encode_transaction_data(
                w3, "initialize", token.abi, args=[owner], kwargs={}
            ),
        ),
        dir="openzeppelin-contracts-upgradeable/lib/openzeppelin-contracts/contracts/proxy/ERC1967",  # noqa: E501
    )
    token2 = await build_and_deploy_contract_async(w3, "MyToken2")
    proxy = w3.eth.contract(address=proxy.address, abi=token.abi)
    hash = await proxy.functions.upgradeToAndCall(token2.address, b"").transact(
        {"from": owner}
    )
    assert (await w3.eth.wait_for_transaction_receipt(hash)).status == 1
    proxy = w3.eth.contract(address=proxy.address, abi=token2.abi)
    assert (await proxy.functions.newFeature().call()) == "Upgraded!"


async def test_storage_layout(evm):
    w3 = evm.async_w3
    acct = ACCOUNTS["validator"]

    short = "Wrap Ether"
    long = "Wrapped Ether Token for testing storage layout" * 32

    artifact = build_contract("WETH9")

    # deploy
    receipt = await send_transaction(
        w3, acct, data=get_initcode(artifact, short, long, 18)
    )
    contract = receipt["contractAddress"]

    # deposit
    await send_transaction(w3, acct, to=contract, value=1000)

    # allowance
    spender = ACCOUNTS["community"].address
    await ERC20.fns.approve(spender, 500).transact(w3, acct, to=contract)

    # name
    slot = await w3.eth.get_storage_at(contract, 0)
    # short string
    assert slot[-1] % 2 == 0
    length = slot[-1] // 2
    name = slot[:length]
    assert short == name.decode()

    # symbol
    slot = await w3.eth.get_storage_at(contract, 1)
    # long string
    assert slot[-1] % 2 == 1
    length = int.from_bytes(slot) >> 1
    assert len(long) == length

    data_slots = (length + 31) // 32
    data_begin = int.from_bytes(keccak((1).to_bytes(32, "big")), "big")
    chunks = []
    for i in range(data_slots):
        s = await w3.eth.get_storage_at(contract, data_begin + i)
        if i == data_slots - 1:
            chunks.append(s[: length - i * 32])
        else:
            chunks.append(s)

    assert long == b"".join(chunks).decode()

    # decimals
    decimals = await w3.eth.get_storage_at(contract, 2)
    assert 18 == int.from_bytes(decimals)

    # balances
    slot = keccak(encode(["address", "uint256"], [acct.address, 3]))
    balance = await w3.eth.get_storage_at(contract, slot)
    assert int.from_bytes(balance, "big") == 1000

    # allowances
    tmp = keccak(encode(["address", "uint256"], [acct.address, 4]))
    slot = keccak(encode(["address", "bytes32"], [spender, tmp]))
    allowance = await w3.eth.get_storage_at(contract, slot)
    assert int.from_bytes(allowance, "big") == 500
