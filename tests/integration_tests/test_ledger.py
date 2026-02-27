from pathlib import Path

import pytest
from pystarport.ledger import Ledger

from .network import setup_custom_evm
from .utils import DEFAULT_DENOM, WEI_PER_DENOM, find_fee

pytestmark = pytest.mark.slow


@pytest.fixture(scope="module")
def custom_evm(request, tmp_path_factory):
    chain = request.config.getoption("chain_config")
    path = tmp_path_factory.mktemp("hw")
    ledger = Ledger()
    try:
        ledger.start()
        assert ledger.is_running(), "failed to start Ledger simulator"

        yield from setup_custom_evm(
            path,
            27300,
            Path(__file__).parent / "configs/hw.jsonnet",
            chain=chain,
        )
    finally:
        try:
            ledger.stop()
        except Exception as e:
            print(f"error during ledger cleanup: {e}")


def test_ledger(custom_evm):
    cli = custom_evm.cosmos_cli()
    name = "hw"
    hw = cli.address(name)
    community = cli.address("community")
    amt1 = 8_000_000_000_000_000_000 // WEI_PER_DENOM
    assert cli.balance(hw) == amt1
    community_balance = cli.balance(community)
    amt2 = 4_000_000_000_000_000_000 // WEI_PER_DENOM
    rsp = cli.transfer(
        hw, community, f"{amt2}{DEFAULT_DENOM}", ledger=True, sign_mode="amino-json"
    )
    assert rsp["code"] == 0, rsp["raw_log"]
    assert cli.balance(hw) == amt2 - find_fee(rsp)
    assert cli.balance(community) == community_balance + amt2

    cli.delete_account(name)

    def check_account(name):
        res = cli.create_account(name, ledger=True, coin_type=118, key_type="secp256k1")
        assert "address" in res
        assert "pubkey" in res
        assert res["type"] == "ledger"
        cli.delete_account(name)

    names = [
        "abc 1",
        r"\&a\)bcd*^",
        "钱對중ガジÑá",
        "this_is_a_very_long_long_long_long_long_long_long_long_long_long_long_long_name",  # noqa: E501
        "1 abc &abcd*^ 钱對중ガジÑá  long_long_long_long_long_long_long_long_long_long_long_long_name",  # noqa: E501
    ]

    for name in names:
        check_account(name)
