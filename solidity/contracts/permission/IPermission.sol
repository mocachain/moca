// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.0;

struct Params {
    uint64 maximumStatementsNum;
    uint64 maximumGroupNum;
    uint64 maximumRemoveExpiredPoliciesIteration;
}

interface IPermission {
    /**
     * @dev params defines a method for queries the parameters of the module.
     */
    function params() external view returns (Params calldata params);
}
