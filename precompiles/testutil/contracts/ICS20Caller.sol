// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../../ics20/ICS20I.sol" as ics20;
import "../../common/Types.sol" as types;

contract ICS20Caller {
    int64 public counter;

    function testIbcTransfer(
        string memory _sourcePort,
        string memory _sourceChannel,
        string memory _denom,
        uint256 _amount,
        address _sender,
        string memory _receiver,
        types.Height memory _timeoutHeight,
        uint64 _timeoutTimestamp,
        string memory _memo
    ) public returns (uint64) {
        return
            ics20.ICS20_CONTRACT.transfer(
            _sourcePort,
            _sourceChannel,
            _denom,
            _amount,
            _sender,
            _receiver,
            _timeoutHeight,
            _timeoutTimestamp,
            _memo
        );
    }

    function testIbcTransferFromContract(
        string memory _sourcePort,
        string memory _sourceChannel,
        string memory _denom,
        uint256 _amount,
        string memory _receiver,
        types.Height memory _timeoutHeight,
        uint64 _timeoutTimestamp,
        string memory _memo
    ) public returns (uint64) {
        return
            ics20.ICS20_CONTRACT.transfer(
            _sourcePort,
            _sourceChannel,
            _denom,
            _amount,
            address(this),
            _receiver,
            _timeoutHeight,
            _timeoutTimestamp,
            _memo
        );
    }

    function testIbcTransferWithTransfer(
        string memory _sourcePort,
        string memory _sourceChannel,
        string memory _denom,
        uint256 _amount,
        address _sender,
        string memory _receiver,
        types.Height memory _timeoutHeight,
        uint64 _timeoutTimestamp,
        string memory _memo,
        bool _before,
        bool _after
    ) public returns (uint64) {
        if (_before) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to sender");
        }
        uint64 nextSequence = ics20.ICS20_CONTRACT.transfer(
            _sourcePort,
            _sourceChannel,
            _denom,
            _amount,
            _sender,
            _receiver,
            _timeoutHeight,
            _timeoutTimestamp,
            _memo
        );
        if (_after) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to sender");
        }
        return nextSequence;
    }

    function testRevertIbcTransfer(
        string memory _sourcePort,
        string memory _sourceChannel,
        string memory _denom,
        uint256 _amount,
        address _sender,
        string memory _receiver,
        address _receiverAddr,
        types.Height memory _timeoutHeight,
        uint64 _timeoutTimestamp,
        string memory _memo,
        bool _after
    ) public {
        try
        ICS20Caller(address(this)).ibcTransferAndRevert(
            _sourcePort,
            _sourceChannel,
            _denom,
            _amount,
            _sender,
            _receiver,
            _timeoutHeight,
            _timeoutTimestamp,
            _memo
        )
        {} catch {}
        if (_after) {
            counter++;
            (bool sent, ) = _receiverAddr.call{value: 15}("");
            require(sent, "Failed to send Ether to delegator");
        }
    }

    function ibcTransferAndRevert(
        string memory _sourcePort,
        string memory _sourceChannel,
        string memory _denom,
        uint256 _amount,
        address _sender,
        string memory _receiver,
        types.Height memory _timeoutHeight,
        uint64 _timeoutTimestamp,
        string memory _memo
    ) external returns (uint64 nextSequence) {
        nextSequence = ics20.ICS20_CONTRACT.transfer(
            _sourcePort,
            _sourceChannel,
            _denom,
            _amount,
            _sender,
            _receiver,
            _timeoutHeight,
            _timeoutTimestamp,
            _memo
        );
        revert();
    }

    function deposit() public payable {}
}