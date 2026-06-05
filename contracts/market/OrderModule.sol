// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./MarketStorage.sol";

abstract contract OrderModule is MarketStorage {
    function createOrder(
        address seller,
        address validator,
        bytes32 requestHash,
        uint256 approvalTimeout
    ) external payable returns (uint256 orderId) {
        if (seller == address(0) || validator == address(0)) revert ZeroAddress();
        if (requestHash == bytes32(0)) revert InvalidHash();
        if (approvalTimeout == 0) revert InvalidTimeout();
        if (!sellers[seller].registered || !sellers[seller].active) revert SellerUnavailable();
        if (!validators[validator].registered || !validators[validator].active) revert ValidatorUnavailable();
        if (!supportsValidator[seller][validator]) revert ValidatorNotSupported();

        uint256 amount = sellers[seller].price;
        bytes32 listingHash = sellers[seller].contentHash;
        if (listingHash == bytes32(0)) revert InvalidHash();

        uint256 validatorFee = validators[validator].fee;
        if (msg.value != amount + validatorFee) revert InvalidPayment();

        orderId = ++orderCount;
        uint256 approvalDeadline = block.timestamp + approvalTimeout;

        orders[orderId] = Order({
            buyer: msg.sender,
            seller: seller,
            validator: validator,
            amount: amount,
            validatorFee: validatorFee,
            listingHash: listingHash,
            requestHash: requestHash,
            deliveryHash: bytes32(0),
            resolutionHash: bytes32(0),
            createdAt: block.timestamp,
            approvalDeadline: approvalDeadline,
            deliveryDeadline: 0,
            responseDeadline: 0,
            status: OrderStatus.PendingSeller
        });

        emit OrderCreated(
            orderId,
            msg.sender,
            seller,
            validator,
            amount,
            validatorFee,
            listingHash,
            requestHash,
            approvalDeadline
        );
    }

    function confirmAsValidator(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.status != OrderStatus.PendingValidator) revert InvalidStatus();
        if (msg.sender != order.validator) revert Unauthorized();
        if (block.timestamp > order.approvalDeadline) revert ApprovalExpired();

        uint256 deliveryTimeout = sellers[order.seller].deliveryTimeout;
        if (deliveryTimeout == 0) revert InvalidTimeout();
        uint256 deliveryDeadline = block.timestamp + deliveryTimeout;

        order.deliveryDeadline = deliveryDeadline;
        order.status = OrderStatus.Created;

        emit ValidatorConfirmed(orderId, deliveryDeadline);
    }

    function confirmAsSeller(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.status != OrderStatus.PendingSeller) revert InvalidStatus();
        if (msg.sender != order.seller) revert Unauthorized();
        if (block.timestamp > order.approvalDeadline) revert ApprovalExpired();

        order.status = OrderStatus.PendingValidator;

        emit SellerConfirmed(orderId);
    }

    function commitDelivery(uint256 orderId, bytes32 deliveryHash) external {
        Order storage order = orders[orderId];
        if (order.status != OrderStatus.Created) revert InvalidStatus();
        if (msg.sender != order.seller) revert Unauthorized();
        if (block.timestamp > order.deliveryDeadline) revert DeliveryExpired();
        if (deliveryHash == bytes32(0)) revert InvalidHash();

        order.deliveryHash = deliveryHash;
        order.status = OrderStatus.DeliveryCommitted;

        emit DeliveryCommitted(orderId, deliveryHash);
    }

    function acceptDelivery(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.status != OrderStatus.DeliveryCommitted) revert InvalidStatus();
        if (msg.sender != order.buyer) revert Unauthorized();

        uint256 amount = order.amount;
        uint256 validatorFee = order.validatorFee;
        address seller = order.seller;
        address buyer = order.buyer;

        order.status = OrderStatus.Released;

        _sendETH(seller, amount);
        _sendETH(buyer, validatorFee);

        emit DeliveryAccepted(orderId);
    }

    function openDispute(uint256 orderId) external {
        Order storage order = orders[orderId];
        bool delivered = order.status == OrderStatus.DeliveryCommitted;
        bool deliveryExpired = order.status == OrderStatus.Created && block.timestamp > order.deliveryDeadline;

        if (!delivered && !deliveryExpired) revert InvalidStatus();

        if (delivered) {
            if (msg.sender != order.buyer && msg.sender != order.seller) revert Unauthorized();
        } else {
            if (msg.sender != order.buyer) revert Unauthorized();
        }

        uint256 responseTimeout = validators[order.validator].responseTimeout;
        uint256 responseDeadline = block.timestamp + responseTimeout;

        order.responseDeadline = responseDeadline;
        order.status = OrderStatus.Disputed;

        emit DisputeOpened(orderId, responseDeadline);
    }

    function resolveDispute(
        uint256 orderId,
        bool releaseToSeller,
        bytes32 resolutionHash
    ) external {
        Order storage order = orders[orderId];
        if (order.status != OrderStatus.Disputed) revert InvalidStatus();
        if (msg.sender != order.validator) revert Unauthorized();
        if (block.timestamp > order.responseDeadline) revert ResolutionExpired();
        if (resolutionHash == bytes32(0)) revert InvalidHash();

        uint256 amount = order.amount;
        uint256 validatorFee = order.validatorFee;
        address buyer = order.buyer;
        address seller = order.seller;
        address validator = order.validator;

        order.resolutionHash = resolutionHash;
        order.status = releaseToSeller ? OrderStatus.ResolvedToSeller : OrderStatus.ResolvedToBuyer;

        _sendETH(releaseToSeller ? seller : buyer, amount);
        _sendETH(validator, validatorFee);

        emit DisputeResolved(orderId, releaseToSeller, resolutionHash);
    }

    function refundIfDeliveryExpired(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.status != OrderStatus.Created) revert InvalidStatus();
        if (block.timestamp <= order.deliveryDeadline) revert DeliveryNotExpired();

        uint256 amount = order.amount;
        uint256 validatorFee = order.validatorFee;
        address buyer = order.buyer;
        address validator = order.validator;

        order.status = OrderStatus.DeliveryExpiredRefunded;

        _sendETH(buyer, amount);
        _sendETH(validator, validatorFee);

        emit DeliveryExpiredRefunded(orderId);
    }

    function refundIfApprovalExpired(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (
            order.status != OrderStatus.PendingValidator &&
            order.status != OrderStatus.PendingSeller
        ) {
            revert InvalidStatus();
        }
        if (block.timestamp <= order.approvalDeadline) revert ApprovalNotExpired();

        uint256 refundAmount = order.amount + order.validatorFee;
        address buyer = order.buyer;

        order.status = OrderStatus.ApprovalExpiredRefunded;

        _sendETH(buyer, refundAmount);

        emit ApprovalExpiredRefunded(orderId);
    }

    function getOrder(uint256 orderId) external view returns (Order memory) {
        return orders[orderId];
    }

    function getOrderCount() external view returns (uint256) {
        return orderCount;
    }

    function _sendETH(address recipient, uint256 amount) private {
        if (amount == 0) return;

        (bool success, ) = recipient.call{value: amount}("");
        if (!success) revert TransferFailed();
    }
}
