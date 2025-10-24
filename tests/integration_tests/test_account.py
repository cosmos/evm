import pytest
import web3

from .utils import derive_new_account


def test_future_blk(evm):
    w3 = evm.w3
    acc = derive_new_account(2).address
    current = w3.eth.block_number
    future = current + 1000
    with pytest.raises(web3.exceptions.Web3RPCError) as exc:
        w3.eth.get_transaction_count(acc, hex(future))
    assert "cannot query with height in the future" in str(exc)
