// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

struct Coin {
    string denom;
    uint256 amount;
}

interface IBank {
    function send(address toAddress, Coin[] calldata amount) external returns (bool success);
}

// EIP-7702 delegation target for test_eip7702.sh: run as a delegated EOA's code, it
// sends the EOA's own amoca through the bank precompile.
contract BankSender {
    IBank private constant BANK = IBank(0x0000000000000000000000000000000000001000);

    function sendAmoca(address to, uint256 amount) external {
        Coin[] memory coins = new Coin[](1);
        coins[0] = Coin({denom: "amoca", amount: amount});
        require(BANK.send(to, coins), "bank send failed");
    }
}
