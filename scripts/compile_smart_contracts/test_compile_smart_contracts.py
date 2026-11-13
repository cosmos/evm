import os
import re
from pathlib import Path
from shutil import copytree

import pytest
from compile_smart_contracts import (
    HARDHAT_PROJECT_DIR,
    SOLIDITY_SOURCE,
    compile_contracts_in_dir,
    copy_to_contracts_directory,
    find_solidity_contracts,
    is_ignored_file,
    is_ignored_folder,
    is_os_repo,
)


@pytest.fixture
def setup_example_contracts_files(tmp_path):
    """
    This fixture creates a temporary folder with some Solidity files.
    """

    (tmp_path / "Contract1.sol").touch()
    (tmp_path / "Contract1.json").touch()
    (tmp_path / "Contract2.sol").touch()
    # NOTE: we're not adding the JSON file for Contract2

    (tmp_path / HARDHAT_PROJECT_DIR).mkdir()
    (tmp_path / HARDHAT_PROJECT_DIR / "Contract3.sol").touch()
    (tmp_path / HARDHAT_PROJECT_DIR / "Contract3.json").touch()

    (tmp_path / "precompiles").mkdir()
    (tmp_path / "precompiles" / "Contract4.sol").touch()
    (tmp_path / "precompiles" / "Contract4.json").touch()

    (tmp_path / "precompiles" / "staking").mkdir(parents=True)
    (tmp_path / "precompiles" / "staking" / "StakingI.sol").touch()
    (tmp_path / "precompiles" / "staking" / "abi.json").touch()

    return tmp_path


def test_find_solidity_files(setup_example_contracts_files):
    tmp_path = setup_example_contracts_files
    found_solidity_contracts = find_solidity_contracts(tmp_path)
    assert len(found_solidity_contracts) == 5

    contract_map = {c.filename: c for c in found_solidity_contracts}

    assert "Contract1" in contract_map
    assert contract_map["Contract1"].path == tmp_path / "Contract1.sol"
    assert contract_map["Contract1"].relative_path == Path(".")
    assert contract_map["Contract1"].compiled_json_path == Path(
        tmp_path / "Contract1.json"
    )

    assert "Contract2" in contract_map
    assert contract_map["Contract2"].path == tmp_path / "Contract2.sol"
    assert contract_map["Contract2"].relative_path == Path(".")
    assert contract_map["Contract2"].compiled_json_path is None

    assert "Contract3" in contract_map
    assert contract_map["Contract3"].relative_path == Path(HARDHAT_PROJECT_DIR)
    assert contract_map["Contract3"].compiled_json_path == Path(
        tmp_path / HARDHAT_PROJECT_DIR / "Contract3.json"
    )

    assert "Contract4" in contract_map
    assert contract_map["Contract4"].path == Path(
        tmp_path / "precompiles" / "Contract4.sol"
    )
    assert contract_map["Contract4"].relative_path == Path("precompiles")
    assert contract_map["Contract4"].compiled_json_path == Path(
        tmp_path / "precompiles" / "Contract4.json"
    )

    assert "StakingI" in contract_map
    assert contract_map["StakingI"].path == Path(
        tmp_path / "precompiles" / "staking" / "StakingI.sol"
    )
    assert contract_map["StakingI"].compiled_json_path == Path(
        tmp_path / "precompiles" / "staking" / "abi.json"
    )


def test_copy_to_contracts_directory(
    tmp_path,
):
    target = tmp_path
    current_dir = Path(os.getcwd())
    assert is_os_repo(
        current_dir
    ), "This test should be executed from the top level of the Cosmos EVM repo"
    contracts = find_solidity_contracts(current_dir)

    assert os.listdir(target) == []
    assert copy_to_contracts_directory(target, contracts) is True

    dir_contents_post = os.listdir(target)
    assert len(dir_contents_post) > 0
    assert os.path.exists(
        target / "precompiles" / "staking" / "testdata" / "StakingCaller.sol"
    )


@pytest.fixture
def setup_contracts_directory(tmp_path):
    """
    This fixture creates a dummy hardhat project from the testdata folder.
    It will serve to test the compilation of smart contracts using this
    script's functions.
    """

    testdata_dir = Path(__file__).parent / "testdata"
    copytree(testdata_dir, tmp_path, dirs_exist_ok=True)

    return tmp_path


def test_compile_contracts_in_dir(setup_contracts_directory):
    hardhat_dir = setup_contracts_directory
    target_dir = hardhat_dir / SOLIDITY_SOURCE

    compile_contracts_in_dir(target_dir)
    assert os.path.exists(
        hardhat_dir
        / "artifacts"
        / SOLIDITY_SOURCE
        / "SimpleContract.sol"
        / "SimpleContract.json"
    )


def test_is_ignored_folder():
    assert is_ignored_folder(f"abc/{HARDHAT_PROJECT_DIR}/{SOLIDITY_SOURCE}") is False
    assert (
        is_ignored_folder(f"abc/{HARDHAT_PROJECT_DIR}/{SOLIDITY_SOURCE}/precompiles")
        is True
    )
    assert (
        is_ignored_folder(f"abc/{HARDHAT_PROJECT_DIR}/{SOLIDITY_SOURCE}/01_example")
        is True
    )
    assert is_ignored_folder("abc/node_modules/precompiles") is True
    assert is_ignored_folder("abc/tests/solidity/precompiles") is True
    assert is_ignored_folder("abc/nix_tests/precompiles") is True


@pytest.mark.parametrize("separator", ["/", "\\"])
def test_ignored_folders_accept_both_path_separators(separator):
    assert is_ignored_folder(separator.join(["abc", "tests", "solidity"]))
    assert is_ignored_folder(
        separator.join(["abc", "contracts", "solidity", "precompiles"])
    )
    assert not is_ignored_folder(separator.join(["abc", "contracts", "solidity"]))


def test_ignored_files_use_portable_paths(tmp_path):
    assert is_ignored_file(tmp_path / "lib" / "vendor" / "Token.sol", tmp_path)
    assert is_ignored_file(tmp_path / "tests" / "integration" / "Token.sol", tmp_path)
    assert not is_ignored_file(tmp_path / "tests" / "contracts" / "Token.sol", tmp_path)


def test_find_solidity_files_excludes_copied_and_test_contracts(tmp_path):
    for name in (
        "precompiles/Token.sol",
        "contracts/solidity/Original.sol",
        "contracts/solidity/precompiles/Copied.sol",
        "tests/solidity/Excluded.sol",
        "tests/integration/Excluded.sol",
        "lib/vendor/Excluded.sol",
        "tests/contracts/Included.sol",
    ):
        contract = tmp_path / name
        contract.parent.mkdir(parents=True, exist_ok=True)
        contract.touch()

    contracts = find_solidity_contracts(tmp_path)
    assert {c.path.relative_to(tmp_path).as_posix() for c in contracts} == {
        "precompiles/Token.sol",
        "contracts/solidity/Original.sol",
        "tests/contracts/Included.sol",
    }


@pytest.mark.parametrize(
    "has_config,return_codes,error_message",
    [
        (False, [], "compilation can only work in a HardHat setup"),
        (True, [1], "Failed to install npm packages."),
        (True, [0, 1], "Failed to compile Solidity contracts."),
        (True, [0, 0], None),
    ],
)
def test_compile_preserves_working_directory(
    tmp_path, monkeypatch, has_config, return_codes, error_message
):
    hardhat_dir = tmp_path / "hardhat"
    hardhat_dir.mkdir()
    if has_config:
        (hardhat_dir / "hardhat.config.js").touch()

    calls = []
    results = iter(return_codes)

    def run_command(command):
        assert Path.cwd() == hardhat_dir
        calls.append(command)
        return next(results)

    monkeypatch.setattr(os, "system", run_command)
    original_dir = Path.cwd()
    try:
        if error_message is None:
            compile_contracts_in_dir(hardhat_dir / SOLIDITY_SOURCE)
        else:
            with pytest.raises(ValueError, match=re.escape(error_message)):
                compile_contracts_in_dir(hardhat_dir / SOLIDITY_SOURCE)
        assert Path.cwd() == original_dir
        assert calls == ["npm install", "npx hardhat compile"][: len(return_codes)]
    finally:
        os.chdir(original_dir)
