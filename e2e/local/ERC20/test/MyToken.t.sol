// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../src/MyToken.sol";

contract MyTokenTest {
    function testTotalSupply() public {
        MyToken token = new MyToken(1_000_000 ether);
        require(token.totalSupply() == 1_000_000 ether, "totalSupply mismatch");
        require(token.balanceOf(address(this)) == 1_000_000 ether, "deployer balance mismatch");
    }

    function testTransfer() public {
        MyToken token = new MyToken(1_000_000 ether);
        address to = address(0xBEEF);
        bool ok = token.transfer(to, 1 ether);
        require(ok, "transfer failed");
        require(token.balanceOf(to) == 1 ether, "recipient balance mismatch");
        require(token.balanceOf(address(this)) == 1_000_000 ether - 1 ether, "sender balance mismatch");
    }
}


