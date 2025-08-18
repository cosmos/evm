// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../../distribution/DistributionI.sol" as distribution;
import "../../gov/IGov.sol" as gov;
import "../../common/Types.sol" as types;

interface IGovCaller {
    function testFundCommunityPool(
        address depositor,
        string memory validatorAddress,
        types.Coin[] memory amount
    ) external returns (bool success);
}


contract GovCaller {
    int64 public counter;

    // Enables ETH to be received with no data
    receive() external payable {}

    function testSubmitProposal(
        address _proposerAddr,
        bytes calldata _jsonProposal,
        types.Coin[] calldata _deposit
    ) public payable returns (uint64 proposalId) {
        return gov.GOV_CONTRACT.submitProposal(
            _proposerAddr,
            _jsonProposal,
            _deposit
        );
    }

    function testSubmitProposalFromContract(
        bytes calldata _jsonProposal,
        types.Coin[] calldata _deposit
    ) public payable returns (uint64 proposalId) {
        return gov.GOV_CONTRACT.submitProposal(
            address(this),
            _jsonProposal,
            _deposit
        );
    }

    function testCancelProposalFromContract(
        uint64 _proposalId
    ) public payable returns (bool success) {
        return gov.GOV_CONTRACT.cancelProposal(
            address(this),
            _proposalId
        );
    }

    function testDeposit(
        address payable _depositorAddr,
        uint64 _proposalId,
        types.Coin[] calldata _deposit
    ) public payable returns (bool success) {
        return gov.GOV_CONTRACT.deposit(
            _depositorAddr,
            _proposalId,
            _deposit
        );
    }

    function testDepositFromContract(
        uint64 _proposalId,
        types.Coin[] calldata _deposit
    ) public payable returns (bool success) {
        return gov.GOV_CONTRACT.deposit(
            address(this),
            _proposalId,
            _deposit
        );
    }

    function testSubmitProposalWithTransfer(
        bytes calldata _jsonProposal,
        types.Coin[] calldata _deposit,
        bool _before,
        bool _after
    ) public payable returns (uint64) {
        if (_before) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        uint64 proposalId = gov.GOV_CONTRACT.submitProposal(
            address(this),
            _jsonProposal,
            _deposit
        );
        if (_after) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        return proposalId;
    }

    function testSubmitProposalFromContractWithTransfer(
        address payable _randomAddr,
        bytes calldata _jsonProposal,
        types.Coin[] calldata _deposit,
        bool _before,
        bool _after
    ) public payable returns (uint64) {
        if (_before) {
            counter++;
            (bool sent, ) = _randomAddr.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        uint64 proposalId = gov.GOV_CONTRACT.submitProposal(
            address(this),
            _jsonProposal,
            _deposit
        );
        if (_after) {
            counter++;
            (bool sent, ) = _randomAddr.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        return proposalId;
    }

    function deposit() public payable {}

    function getParams() external view returns (gov.Params memory params) {
        return gov.GOV_CONTRACT.getParams();
    }

    function testDepositWithTransfer(
        uint64 _proposalId,
        types.Coin[] calldata _deposit,
        bool _before,
        bool _after
    ) public payable returns (bool success) {
        if (_before) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        success = gov.GOV_CONTRACT.deposit(
            address(this),
            _proposalId,
            _deposit
        );
        if (_after) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
    }

    function testDepositFromContractWithTransfer(
        address payable _randomAddr,
        uint64 _proposalId,
        types.Coin[] calldata _deposit,
        bool _before,
        bool _after
    ) public payable returns (bool success) {
        if (_before) {
            counter++;
            (bool sent, ) = _randomAddr.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        success = gov.GOV_CONTRACT.deposit(
            address(this),
            _proposalId,
            _deposit
        );
        if (_after) {
            counter++;
            (bool sent, ) = _randomAddr.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
    }

    function testCancelWithTransfer(
        uint64 _proposalId,
        bool _before,
        bool _after
    ) public payable returns (bool success) {
        if (_before) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        success = gov.GOV_CONTRACT.cancelProposal(
            address(this),
            _proposalId
        );
        if (_after) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
    }

    function testCancelFromContractWithTransfer(
        address payable _randomAddr,
        uint64 _proposalId,
        bool _before,
        bool _after
    ) public payable returns (bool success) {
        if (_before) {
            counter++;
            (bool sent, ) = _randomAddr.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
        success = gov.GOV_CONTRACT.cancelProposal(
            address(this),
            _proposalId
        );
        if (_after) {
            counter++;
            (bool sent, ) = _randomAddr.call{value: 15}("");
            require(sent, "Failed to send Ether to proposer");
        }
    }

    function testTransferCancelFund(
        address payable depositor,
        uint64 _proposalId,
        bytes calldata denom,
        string memory validatorAddress
    ) public payable returns (bool success) {
        IGovCaller govDepositor = IGovCaller(depositor);
        counter++;
        // Send 1 wei to depositor
        (bool sent, ) = depositor.call{value: 1}("");
        require(sent, "Failed to send Ether to depositor");
        // Cancel the Proposal
        try gov.GOV_CONTRACT.cancelProposal(address(this), _proposalId) returns (bool res) {
            require(res, "cancelProposal returned false");
        } catch Error(string memory reason) {
            revert(string(abi.encodePacked("cancelProposal failed: ", reason)));
        } catch {
            revert("cancelProposal failed silently");
        }
        // Deposit 2 wei to validator pool from proposer
        counter++;
        types.Coin[] memory coins = new types.Coin[](1);
        coins[0] = types.Coin(string(denom), 2);
        try govDepositor.testFundCommunityPool(address(depositor), validatorAddress, coins) returns (bool res) {
            require(res, "fundCommunityPool returned false");
        } catch Error(string memory reason) {
            revert(string(abi.encodePacked("fundCommunityPool failed: ", reason)));
        } catch {
            revert("fundCommunityPool failed silently");
        }
        success = true;
    }

    /// @dev testFundCommunityPool defines a method to allow an account to directly
    /// fund the community pool.
    /// @param depositor The address of the depositor
    /// @param amount The amount of coin fund community pool
    /// @return success Whether the transaction was successful or not
    function testFundCommunityPool(
        address depositor,
        string memory validatorAddress,
        types.Coin[] memory amount
    ) public returns (bool success) {
        counter += 1;
        success = distribution.DISTRIBUTION_CONTRACT.depositValidatorRewardsPool(
            depositor,
            validatorAddress,
            amount
        );
        counter -= 1;
        return success;
    }
}
