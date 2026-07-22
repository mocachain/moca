// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// Minimal EIP-7702 delegation-target fixture (test_eip7702.sh). TestERC20's
// mint/burn are owner-gated on a constructor-set `owner`, which never runs
// against a delegated EOA's own (blank) storage -- msg.sender would never
// equal the always-zero owner slot there, so every call reverts. Counter has
// no such gating: inc() is callable by anyone and records who called it.
contract Counter {
    uint256 public count;
    address public lastCaller;

    function inc() external {
        count += 1;
        lastCaller = msg.sender;
    }
}
