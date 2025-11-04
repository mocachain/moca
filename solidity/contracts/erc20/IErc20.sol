// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.0;

import "../common/Types.sol";

/**
 * @dev Owner enumerates the ownership of a ERC20 contract
 */
enum Owner {
    // OWNER_UNSPECIFIED defines an invalid/undefined owner
    OWNER_UNSPECIFIED,
    // OWNER_MODULE - erc20 is owned by the erc20 module account
    OWNER_MODULE,
    // OWNER_EXTERNAL - erc20 is owned by an external account
    OWNER_EXTERNAL
}

/**
 * @dev TokenPair defines an instance that records a pairing consisting of a native
 * Cosmos Coin and an ERC20 token address
 */
struct TokenPair {
    // erc20_address is the hex address of ERC20 contract token
    address erc20Address;
    // denom defines the cosmos base denomination to be mapped to
    string denom;
    // enabled defines the token mapping enable status
    bool enabled;
    // contract_owner is the an ENUM specifying the type of ERC20 owner
    Owner contractOwner;
}

/**
 * @dev Params defines the parameters for the erc20 module.
 */
struct Params {
    bool enableErc20;
    bool enableEvmHook;
}

interface IErc20 {
    /**
     * @dev Convert native Cosmos coin to ERC20 token
     * @param coin The Cosmos coin information to convert
     * @param receiver The address to receive ERC20 tokens
     */
    function convertCoin(
        Coin calldata coin,
        address receiver
    ) external returns (bool);

    /**
     * @dev Convert ERC20 token to native Cosmos coin
     * @param contractAddress The ERC20 token contract address
     * @param amount The amount of ERC20 tokens to convert
     * @param receiver The bech32 address to receive Cosmos coins
     */
    function convertERC20(
        address contractAddress,
        uint256 amount,
        address receiver
    ) external returns (bool);

    /**
     * @dev Retrieves registered token pairs with pagination
     * @param pagination Pagination parameters for the request
     * @return tokenPairs Array of registered token pairs
     * @return pageResponse Pagination information in the response
     */
    function tokenPairs(
        PageRequest calldata pagination
    )
        external
        view
        returns (
            TokenPair[] memory tokenPairs,
            PageResponse memory pageResponse
        );

    /**
     * @dev Retrieves a specific registered token pair
     * @param token Token identifier (hex contract address or Cosmos base denomination)
     * @return tokenPair Information about the registered token pair
     */
    function tokenPair(
        address token
    ) external view returns (TokenPair memory tokenPair);

    /**
     * @dev Retrieves the erc20 module parameters
     * @return params The current erc20 module parameters
     */
    function params() external view returns (Params memory params);

    /**
     * @dev ConvertCoin defines an Event emitted when coins are converted to ERC20 tokens
     */
    event ConvertCoin(
        address indexed receiver,
        address indexed sender,
        string denom,
        uint256 amount
    );

    /**
     * @dev ConvertERC20 defines an Event emitted when ERC20 tokens are converted to native coins
     */
    event ConvertERC20(
        address indexed contractAddress,
        address indexed receiver,
        address indexed sender,
        uint256 amount
    );
}
