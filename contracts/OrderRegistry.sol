// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface ISellerRegistry {
    function isSellerActive(address seller) external view returns (bool);
    function getSellerPrice(address seller) external view returns (uint256);
    function getSellerContentHash(address seller) external view returns (bytes32);
    function getSellerDeliveryTimeout(address seller) external view returns (uint256);
    function isValidatorSupported(address seller, address validator) external view returns (bool);
}

interface IValidatorRegistry {
    function isValidatorActive(address validator) external view returns (bool);
    function getValidator(address validator)
        external
        view
        returns (
            bool registered,
            bool active,
            string memory validatorURI,
            uint256 fee,
            uint256 responseTimeout
        );
}

contract OrderRegistry {
    enum OrderStatus {
        None,
        PendingSeller,
        PendingValidator,
        Created,
        DeliveryCommitted,
        Disputed,
        Released,
        ApprovalExpiredRefunded,
        DeliveryExpiredRefunded,
        ResolvedToSeller,
        ResolvedToBuyer
    }

    struct Order {
        address buyer;
        address seller;
        address validator;
        uint256 amount;
        uint256 validatorFee;
        bytes32 listingHash;
        bytes32 requestHash;
        bytes32 deliveryHash;
        bytes32 resolutionHash;
        uint256 createdAt;
        uint256 approvalDeadline;
        uint256 deliveryDeadline;
        uint256 responseDeadline;
        OrderStatus status;
    }

    error ZeroAddress();
    error InvalidTimeout();
    error InvalidPayment();
    error InvalidHash();
    error InvalidStatus();
    error Unauthorized();
    error SellerUnavailable();
    error ValidatorUnavailable();
    error ValidatorNotSupported();
    error DeliveryExpired();
    error DeliveryNotExpired();
    error ApprovalExpired();
    error ApprovalNotExpired();
    error ResolutionExpired();
    error TransferFailed();

    event OrderCreated(
        uint256 indexed orderId,
        address indexed buyer,
        address indexed seller,
        address validator,
        uint256 amount,
        uint256 validatorFee,
        bytes32 listingHash,
        bytes32 requestHash,
        uint256 approvalDeadline
    );
    event SellerConfirmed(uint256 indexed orderId);
    event ValidatorConfirmed(uint256 indexed orderId, uint256 deliveryDeadline);
    event DeliveryCommitted(
        uint256 indexed orderId,
        bytes32 deliveryHash
    );
    event DeliveryAccepted(uint256 indexed orderId);
    event DisputeOpened(uint256 indexed orderId, uint256 responseDeadline);
    event DisputeResolved(uint256 indexed orderId, bool releaseToSeller, bytes32 resolutionHash);
    event ApprovalExpiredRefunded(uint256 indexed orderId);
    event DeliveryExpiredRefunded(uint256 indexed orderId);

    ISellerRegistry public immutable sellerRegistry;
    IValidatorRegistry public immutable validatorRegistry;

    uint256 private orderCount;
    mapping(uint256 => Order) private orders;

    constructor(address sellerRegistry_, address validatorRegistry_) {
        if (sellerRegistry_ == address(0) || validatorRegistry_ == address(0)) {
            revert ZeroAddress();
        }

        sellerRegistry = ISellerRegistry(sellerRegistry_);
        validatorRegistry = IValidatorRegistry(validatorRegistry_);
    }

    function createOrder(
        address seller,
        address validator,
        bytes32 requestHash,
        uint256 approvalTimeout
    ) external payable returns (uint256 orderId) {
        if (seller == address(0) || validator == address(0)) revert ZeroAddress();
        if (requestHash == bytes32(0)) revert InvalidHash();
        if (approvalTimeout == 0) revert InvalidTimeout();
        if (!sellerRegistry.isSellerActive(seller)) revert SellerUnavailable();
        if (!validatorRegistry.isValidatorActive(validator)) revert ValidatorUnavailable();
        if (!sellerRegistry.isValidatorSupported(seller, validator)) revert ValidatorNotSupported();

        uint256 amount = sellerRegistry.getSellerPrice(seller);
        bytes32 listingHash = sellerRegistry.getSellerContentHash(seller);
        if (listingHash == bytes32(0)) revert InvalidHash();

        (, , , uint256 validatorFee, ) = validatorRegistry.getValidator(validator);
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

        uint256 deliveryTimeout = sellerRegistry.getSellerDeliveryTimeout(order.seller);
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

    function commitDelivery(
        uint256 orderId,
        bytes32 deliveryHash
    ) external {
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

        (, , , , uint256 responseTimeout) = validatorRegistry.getValidator(order.validator);
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
