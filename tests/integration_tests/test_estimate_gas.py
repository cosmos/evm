from .utils import (
    AsyncTestMessageCall,
    AsyncTestRevert,
    create_contract_transaction,
)

METHOD = "eth_estimateGas"


async def test_revert(evm):
    w3 = evm.async_w3
    revert = AsyncTestRevert()
    await revert.deploy(w3)
    data = "0x9ffb86a5"
    params = {"to": revert.address, "data": data}
    rsp = await w3.provider.make_request(METHOD, [params])
    error = rsp["error"]
    assert error["code"] == 3
    assert error["message"] == "execution reverted: Function has been reverted"
    assert (
        error["data"]
        == "0x08c379a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000001a46756e6374696f6e20686173206265656e207265766572746564000000000000"  # noqa: E501
    )


async def test_out_of_gas_error(evm):
    iterations = 1
    gas = 21204
    w3 = evm.async_w3
    msg = AsyncTestMessageCall()
    await msg.deploy(w3)
    tx_data = msg.get_test_data(iterations)
    tx_params = {"to": msg.address, "data": tx_data, "gas": hex(gas)}
    rsp = await w3.provider.make_request(METHOD, [tx_params])
    error = rsp["error"]
    assert error["code"] == -32000
    assert f"gas required exceeds allowance ({gas})" in error["message"]


async def test_storage_out_of_gas_error(evm):
    gas = 210000
    w3 = evm.async_w3
    tx = await create_contract_transaction(w3, "TestMessageCall")
    tx_params = {"data": tx["data"], "gas": hex(gas)}
    rsp = await w3.provider.make_request(METHOD, [tx_params])
    error = rsp["error"]
    assert error["code"] == -32000
    assert "contract creation code storage out of gas" in error["message"]
