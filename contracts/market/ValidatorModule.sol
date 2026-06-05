// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./MarketStorage.sol";

abstract contract ValidatorModule is MarketStorage {
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
