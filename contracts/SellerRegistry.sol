// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract SellerRegistry {
    struct Seller {
        bool registered;
        bool active;
        string sellerURI;
        uint256 price;
        string contentURI;
        bytes32 contentHash;
        uint256 deliveryTimeout;
    }

    error AlreadyRegistered();
    error NotRegistered();
    error ZeroAddress();
    error InvalidContentHash();
    error InvalidDeliveryTimeout();
    error ValidatorAlreadySupported();
    error ValidatorNotSupported();

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

    mapping(address => Seller) private sellers;
    mapping(address => address[]) private sellerValidators;
    mapping(address => mapping(address => bool)) private supportsValidator;
    address[] private sellerList;

    modifier onlyRegisteredSeller() {
        if (!sellers[msg.sender].registered) revert NotRegistered();
        _;
    }

    function registerSeller(
        string calldata sellerURI,
        uint256 price,
        string calldata contentURI,
        bytes32 contentHash,
        uint256 deliveryTimeout
    ) external {
        Seller storage seller = sellers[msg.sender];
        if (seller.registered) revert AlreadyRegistered();
        if (contentHash == bytes32(0)) revert InvalidContentHash();
        if (deliveryTimeout == 0) revert InvalidDeliveryTimeout();

        sellers[msg.sender] = Seller({
            registered: true,
            active: true,
            sellerURI: sellerURI,
            price: price,
            contentURI: contentURI,
            contentHash: contentHash,
            deliveryTimeout: deliveryTimeout
        });
        sellerList.push(msg.sender);

        emit SellerRegistered(msg.sender, sellerURI);
        emit SellerProductUpdated(msg.sender, price, contentURI, contentHash, deliveryTimeout);
    }

    function setSellerURI(string calldata sellerURI) external onlyRegisteredSeller {
        sellers[msg.sender].sellerURI = sellerURI;
        emit SellerURIUpdated(msg.sender, sellerURI);
    }

    function setSellerActive(bool active) external onlyRegisteredSeller {
        sellers[msg.sender].active = active;
        emit SellerActiveUpdated(msg.sender, active);
    }

    function setProduct(
        uint256 price,
        string calldata contentURI,
        bytes32 contentHash,
        uint256 deliveryTimeout
    ) external onlyRegisteredSeller {
        if (contentHash == bytes32(0)) revert InvalidContentHash();
        if (deliveryTimeout == 0) revert InvalidDeliveryTimeout();

        Seller storage seller = sellers[msg.sender];
        seller.price = price;
        seller.contentURI = contentURI;
        seller.contentHash = contentHash;
        seller.deliveryTimeout = deliveryTimeout;

        emit SellerProductUpdated(msg.sender, price, contentURI, contentHash, deliveryTimeout);
    }

    function addSupportedValidator(address validator) external onlyRegisteredSeller {
        if (validator == address(0)) revert ZeroAddress();
        if (supportsValidator[msg.sender][validator]) revert ValidatorAlreadySupported();

        supportsValidator[msg.sender][validator] = true;
        sellerValidators[msg.sender].push(validator);

        emit ValidatorAdded(msg.sender, validator);
    }

    function removeSupportedValidator(address validator) external onlyRegisteredSeller {
        if (!supportsValidator[msg.sender][validator]) revert ValidatorNotSupported();

        supportsValidator[msg.sender][validator] = false;

        address[] storage validators = sellerValidators[msg.sender];
        uint256 validatorCount = validators.length;
        for (uint256 i; i < validatorCount; i++) {
            if (validators[i] == validator) {
                validators[i] = validators[validatorCount - 1];
                validators.pop();
                break;
            }
        }

        emit ValidatorRemoved(msg.sender, validator);
    }

    function isSeller(address seller) external view returns (bool) {
        return sellers[seller].registered;
    }

    function isSellerActive(address seller) external view returns (bool) {
        return sellers[seller].registered && sellers[seller].active;
    }

    function getSeller(address seller)
        external
        view
        returns (
            bool registered,
            bool active,
            string memory sellerURI,
            uint256 price,
            string memory contentURI,
            bytes32 contentHash,
            uint256 deliveryTimeout
        )
    {
        Seller storage sellerData = sellers[seller];
        return (
            sellerData.registered,
            sellerData.active,
            sellerData.sellerURI,
            sellerData.price,
            sellerData.contentURI,
            sellerData.contentHash,
            sellerData.deliveryTimeout
        );
    }

    function getSellers() external view returns (address[] memory) {
        return sellerList;
    }

    function getSellerPrice(address seller) external view returns (uint256) {
        return sellers[seller].price;
    }

    function getSellerContentHash(address seller) external view returns (bytes32) {
        return sellers[seller].contentHash;
    }

    function getSellerDeliveryTimeout(address seller) external view returns (uint256) {
        return sellers[seller].deliveryTimeout;
    }

    function isValidatorSupported(address seller, address validator) external view returns (bool) {
        return supportsValidator[seller][validator];
    }

    function getSupportedValidators(address seller) external view returns (address[] memory) {
        return sellerValidators[seller];
    }
}
