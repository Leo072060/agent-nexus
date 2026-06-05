// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

abstract contract MarketStorage {
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

    struct Seller {
        bool registered;
        bool active;
        string sellerURI;
        uint256 price;
        string contentURI;
        bytes32 contentHash;
        uint256 deliveryTimeout;
    }

    struct Validator {
        bool registered;
        bool active;
        string validatorURI;
        uint256 fee;
        uint256 responseTimeout;
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

    error AlreadyRegistered();
    error NotRegistered();
    error ZeroAddress();
    error InvalidContentHash();
    error InvalidDeliveryTimeout();
    error ValidatorAlreadySupported();
    error ValidatorNotSupported();
    error InvalidResponseTimeout();
    error InvalidTimeout();
    error InvalidPayment();
    error InvalidHash();
    error InvalidStatus();
    error Unauthorized();
    error SellerUnavailable();
    error ValidatorUnavailable();
    error DeliveryExpired();
    error DeliveryNotExpired();
    error ApprovalExpired();
    error ApprovalNotExpired();
    error ResolutionExpired();
    error TransferFailed();

    event SellerRegistered(address indexed seller, string sellerURI);
    event SellerURIUpdated(address indexed seller, string sellerURI);
    event SellerActiveUpdated(address indexed seller, bool active);
    event SellerProductUpdated(
        address indexed seller,
        uint256 price,
        string contentURI,
        bytes32 contentHash,
        uint256 deliveryTimeout
    );
    event ValidatorAdded(address indexed seller, address indexed validator);
    event ValidatorRemoved(address indexed seller, address indexed validator);

    event ValidatorRegistered(
        address indexed validator,
        string validatorURI,
        uint256 fee,
        uint256 responseTimeout
    );
    event ValidatorURIUpdated(address indexed validator, string validatorURI);
    event ValidatorActiveUpdated(address indexed validator, bool active);
    event ValidatorFeeUpdated(address indexed validator, uint256 fee);
    event ValidatorResponseTimeoutUpdated(address indexed validator, uint256 responseTimeout);

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
    event DeliveryCommitted(uint256 indexed orderId, bytes32 deliveryHash);
    event DeliveryAccepted(uint256 indexed orderId);
    event DisputeOpened(uint256 indexed orderId, uint256 responseDeadline);
    event DisputeResolved(uint256 indexed orderId, bool releaseToSeller, bytes32 resolutionHash);
    event ApprovalExpiredRefunded(uint256 indexed orderId);
    event DeliveryExpiredRefunded(uint256 indexed orderId);

    mapping(address => Seller) internal sellers;
    mapping(address => address[]) internal sellerValidators;
    mapping(address => mapping(address => bool)) internal supportsValidator;
    address[] internal sellerList;

    mapping(address => Validator) internal validators;
    address[] internal validatorList;

    uint256 internal orderCount;
    mapping(uint256 => Order) internal orders;
}
