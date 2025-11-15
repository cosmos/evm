import pytest

from .network import setup_evm


def pytest_addoption(parser):
    parser.addoption(
        "--chain-config",
        default="evmd",
        action="store",
        metavar="CHAIN_CONFIG",
        required=False,
        help="Specify chain config to test",
    )


def pytest_configure(config):
    config.addinivalue_line("markers", "unmarked: fallback mark for unmarked tests")
    config.addinivalue_line("markers", "slow: marks tests as slow")
    config.addinivalue_line("markers", "asyncio: marks tests as asyncio")


@pytest.fixture(scope="session")
def suspend_capture(pytestconfig):
    """
    used to pause in testing

    Example:
    ```
    def test_simple(suspend_capture):
        with suspend_capture:
            # read user input
            print(input())
    ```
    """

    class SuspendGuard:
        def __init__(self):
            self.capmanager = pytestconfig.pluginmanager.getplugin("capturemanager")

        def __enter__(self):
            self.capmanager.suspend_global_capture(in_=True)

        def __exit__(self, _1, _2, _3):
            self.capmanager.resume_global_capture()

    yield SuspendGuard()


@pytest.fixture(scope="session", params=[True])
def evm(request, tmp_path_factory):
    chain = request.config.getoption("chain_config")
    path = tmp_path_factory.mktemp("evm")
    yield from setup_evm(path, 26650, chain)
