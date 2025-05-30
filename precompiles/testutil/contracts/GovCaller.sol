// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../../gov/IGov.sol" as gov;
import "../../common/Types.sol" as types;

contract GovCaller {
    int64 public counter;

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
}
