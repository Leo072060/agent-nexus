// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./market/SellerModule.sol";
import "./market/ValidatorModule.sol";
import "./market/OrderModule.sol";

contract Market is SellerModule, ValidatorModule, OrderModule {
    address public immutable owner;
    string public marketURI;

    event MarketURIUpdated(string marketURI);

    constructor(string memory marketURI_) {
        owner = msg.sender;
        marketURI = marketURI_;
        emit MarketURIUpdated(marketURI_);
    }

    function setMarketURI(string calldata marketURI_) external {
        if (msg.sender != owner) revert Unauthorized();

        marketURI = marketURI_;
        emit MarketURIUpdated(marketURI_);
    }
}
