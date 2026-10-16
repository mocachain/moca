// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// EIP-7702 delegation target for test_eip7702.sh. No owner gate (unlike TestERC20),
// since a delegated EOA's blank storage never holds a constructor-set owner.
contract Counter {
    uint256 public count;
    address public lastCaller;

    function inc() external {
        count += 1;
        lastCaller = msg.sender;
    }
}
