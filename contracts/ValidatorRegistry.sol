// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract ValidatorRegistry {
    struct Validator {
        bool registered;
        bool active;
        string validatorURI;
        uint256 fee;
        uint256 responseTimeout;
    }

    error AlreadyRegistered();
    error NotRegistered();
    error InvalidResponseTimeout();

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

    mapping(address => Validator) private validators;
    address[] private validatorList;

    modifier onlyRegisteredValidator() {
        if (!validators[msg.sender].registered) revert NotRegistered();
        _;
    }

    function registerValidator(
        string calldata validatorURI,
        uint256 fee,
        uint256 responseTimeout
    ) external {
        if (validators[msg.sender].registered) revert AlreadyRegistered();
        if (responseTimeout == 0) revert InvalidResponseTimeout();

        validators[msg.sender] = Validator({
            registered: true,
            active: true,
            validatorURI: validatorURI,
            fee: fee,
            responseTimeout: responseTimeout
        });
        validatorList.push(msg.sender);

        emit ValidatorRegistered(msg.sender, validatorURI, fee, responseTimeout);
    }

    function setValidatorURI(string calldata validatorURI) external onlyRegisteredValidator {
        validators[msg.sender].validatorURI = validatorURI;
        emit ValidatorURIUpdated(msg.sender, validatorURI);
    }

    function setValidatorActive(bool active) external onlyRegisteredValidator {
        validators[msg.sender].active = active;
        emit ValidatorActiveUpdated(msg.sender, active);
    }

    function setValidatorFee(uint256 fee) external onlyRegisteredValidator {
        validators[msg.sender].fee = fee;
        emit ValidatorFeeUpdated(msg.sender, fee);
    }

    function setResponseTimeout(uint256 responseTimeout) external onlyRegisteredValidator {
        if (responseTimeout == 0) revert InvalidResponseTimeout();

        validators[msg.sender].responseTimeout = responseTimeout;
        emit ValidatorResponseTimeoutUpdated(msg.sender, responseTimeout);
    }

    function isValidator(address validator) external view returns (bool) {
        return validators[validator].registered;
    }

    function isValidatorActive(address validator) external view returns (bool) {
        return validators[validator].registered && validators[validator].active;
    }

    function getValidator(address validator)
        external
        view
        returns (
            bool registered,
            bool active,
            string memory validatorURI,
            uint256 fee,
            uint256 responseTimeout
        )
    {
        Validator storage validatorData = validators[validator];
        return (
            validatorData.registered,
            validatorData.active,
            validatorData.validatorURI,
            validatorData.fee,
            validatorData.responseTimeout
        );
    }

    function getValidators() external view returns (address[] memory) {
        return validatorList;
    }
}
