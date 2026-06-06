// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../contracts/Market.sol";
import "../contracts/market/MarketStorage.sol";

interface Vm {
    function addr(uint256 privateKey) external returns (address);
    function deal(address who, uint256 newBalance) external;
    function expectRevert() external;
    function prank(address msgSender) external;
    function warp(uint256 newTimestamp) external;
}

contract MarketTest {
    Vm private constant vm = Vm(address(uint160(uint256(keccak256("hevm cheat code")))));

    uint256 private constant PRICE = 1 ether;
    uint256 private constant VALIDATOR_FEE = 0.1 ether;
    uint256 private constant DELIVERY_TIMEOUT = 1 days;
    uint256 private constant RESPONSE_TIMEOUT = 1 days;
    uint256 private constant APPROVAL_TIMEOUT = 1 hours;

    Market private market;
    address private buyer;
    address private seller;
    address private validator;
    address private outsider;

    bytes32 private contentHash = keccak256("content-v1");
    bytes32 private requestHash = keccak256("request-v1");
    bytes32 private deliveryHash = keccak256("delivery-v1");
    bytes32 private resolutionHash = keccak256("resolution-v1");

    receive() external payable {}

    function setUp() public {
        market = new Market("ipfs://market");
        buyer = vm.addr(1);
        seller = vm.addr(2);
        validator = vm.addr(3);
        outsider = vm.addr(4);

        vm.deal(buyer, 100 ether);
        vm.deal(seller, 100 ether);
        vm.deal(validator, 100 ether);
        vm.deal(outsider, 100 ether);
    }

    function testHappyPathReleasesSellerAndRefundsValidatorFeeToBuyer() public {
        _registerSellerAndValidator();
        uint256 buyerBefore = buyer.balance;
        uint256 sellerBefore = seller.balance;

        uint256 orderId = _createOrder();
        _confirmSeller(orderId);
        _confirmValidator(orderId);
        _commitDelivery(orderId);

        vm.prank(buyer);
        market.acceptDelivery(orderId);

        MarketStorage.Order memory order = market.getOrder(orderId);
        _assertEq(uint256(order.status), uint256(MarketStorage.OrderStatus.Released), "released status");
        _assertEq(seller.balance, sellerBefore + PRICE, "seller received price");
        _assertEq(buyer.balance, buyerBefore - PRICE, "buyer only paid price");
        _assertEq(address(market).balance, 0, "market balance cleared");
    }

    function testBuyerCanOpenDisputeAndValidatorCanResolveToSeller() public {
        uint256 orderId = _createCommittedOrder();

        vm.prank(buyer);
        market.openDispute(orderId);

        uint256 sellerBefore = seller.balance;
        uint256 validatorBefore = validator.balance;

        vm.prank(validator);
        market.resolveDispute(orderId, true, resolutionHash);

        MarketStorage.Order memory order = market.getOrder(orderId);
        _assertEq(uint256(order.status), uint256(MarketStorage.OrderStatus.ResolvedToSeller), "seller resolution status");
        _assertEq(seller.balance, sellerBefore + PRICE, "seller received price");
        _assertEq(validator.balance, validatorBefore + VALIDATOR_FEE, "validator received fee");
        _assertEq(order.resolutionHash, resolutionHash, "resolution hash saved");
        _assertEq(address(market).balance, 0, "market balance cleared");
    }

    function testSellerCanOpenDisputeAndValidatorCanResolveToBuyer() public {
        uint256 orderId = _createCommittedOrder();

        vm.prank(seller);
        market.openDispute(orderId);

        uint256 buyerBefore = buyer.balance;
        uint256 validatorBefore = validator.balance;

        vm.prank(validator);
        market.resolveDispute(orderId, false, resolutionHash);

        MarketStorage.Order memory order = market.getOrder(orderId);
        _assertEq(uint256(order.status), uint256(MarketStorage.OrderStatus.ResolvedToBuyer), "buyer resolution status");
        _assertEq(buyer.balance, buyerBefore + PRICE, "buyer received price refund");
        _assertEq(validator.balance, validatorBefore + VALIDATOR_FEE, "validator received fee");
        _assertEq(address(market).balance, 0, "market balance cleared");
    }

    function testApprovalExpiryRefundsBuyerBeforeFinalApproval() public {
        _registerSellerAndValidator();
        uint256 buyerBefore = buyer.balance;

        uint256 orderId = _createOrder();
        vm.warp(block.timestamp + APPROVAL_TIMEOUT + 1);

        market.refundIfApprovalExpired(orderId);

        MarketStorage.Order memory order = market.getOrder(orderId);
        _assertEq(uint256(order.status), uint256(MarketStorage.OrderStatus.ApprovalExpiredRefunded), "approval refund status");
        _assertEq(buyer.balance, buyerBefore, "buyer fully refunded");
        _assertEq(address(market).balance, 0, "market balance cleared");
    }

    function testDeliveryExpiryRefundsBuyerAndPaysValidator() public {
        _registerSellerAndValidator();
        uint256 orderId = _createOrder();
        _confirmSeller(orderId);
        _confirmValidator(orderId);

        uint256 buyerBefore = buyer.balance;
        uint256 validatorBefore = validator.balance;

        vm.warp(block.timestamp + DELIVERY_TIMEOUT + 1);
        market.refundIfDeliveryExpired(orderId);

        MarketStorage.Order memory order = market.getOrder(orderId);
        _assertEq(uint256(order.status), uint256(MarketStorage.OrderStatus.DeliveryExpiredRefunded), "delivery refund status");
        _assertEq(buyer.balance, buyerBefore + PRICE, "buyer received price refund");
        _assertEq(validator.balance, validatorBefore + VALIDATOR_FEE, "validator received fee");
        _assertEq(address(market).balance, 0, "market balance cleared");
    }

    function testBuyerCanOpenDisputeAfterDeliveryExpiry() public {
        _registerSellerAndValidator();
        uint256 orderId = _createOrder();
        _confirmSeller(orderId);
        _confirmValidator(orderId);

        vm.warp(block.timestamp + DELIVERY_TIMEOUT + 1);

        vm.prank(buyer);
        market.openDispute(orderId);

        MarketStorage.Order memory order = market.getOrder(orderId);
        _assertEq(uint256(order.status), uint256(MarketStorage.OrderStatus.Disputed), "disputed status");
    }

    function testValidatorCannotResolveAfterResponseDeadline() public {
        uint256 orderId = _createCommittedOrder();

        vm.prank(buyer);
        market.openDispute(orderId);
        vm.warp(block.timestamp + RESPONSE_TIMEOUT + 1);

        vm.expectRevert();
        vm.prank(validator);
        market.resolveDispute(orderId, false, resolutionHash);
    }

    function testCreateOrderRejectsUnavailableActorsUnsupportedValidatorBadHashAndPayment() public {
        vm.expectRevert();
        vm.prank(buyer);
        market.createOrder{value: PRICE + VALIDATOR_FEE}(seller, validator, requestHash, APPROVAL_TIMEOUT);

        _registerSellerAndValidatorWithoutSupport();
        vm.expectRevert();
        vm.prank(buyer);
        market.createOrder{value: PRICE + VALIDATOR_FEE}(seller, validator, requestHash, APPROVAL_TIMEOUT);

        vm.prank(seller);
        market.addSupportedValidator(validator);

        vm.expectRevert();
        vm.prank(buyer);
        market.createOrder{value: PRICE + VALIDATOR_FEE}(seller, validator, bytes32(0), APPROVAL_TIMEOUT);

        vm.expectRevert();
        vm.prank(buyer);
        market.createOrder{value: PRICE + VALIDATOR_FEE - 1}(seller, validator, requestHash, APPROVAL_TIMEOUT);

        vm.prank(seller);
        market.setSellerActive(false);
        vm.expectRevert();
        vm.prank(buyer);
        market.createOrder{value: PRICE + VALIDATOR_FEE}(seller, validator, requestHash, APPROVAL_TIMEOUT);

        vm.prank(seller);
        market.setSellerActive(true);
        vm.prank(validator);
        market.setValidatorActive(false);
        vm.expectRevert();
        vm.prank(buyer);
        market.createOrder{value: PRICE + VALIDATOR_FEE}(seller, validator, requestHash, APPROVAL_TIMEOUT);
    }

    function testRegistrationRejectsZeroHashesAndTimeouts() public {
        vm.expectRevert();
        vm.prank(seller);
        market.registerSeller("http://seller", PRICE, "http://content", bytes32(0), DELIVERY_TIMEOUT);

        vm.expectRevert();
        vm.prank(seller);
        market.registerSeller("http://seller", PRICE, "http://content", contentHash, 0);

        vm.expectRevert();
        vm.prank(validator);
        market.registerValidator("http://validator", VALIDATOR_FEE, 0);
    }

    function testOnlyAuthorizedActorsCanAdvanceOrder() public {
        _registerSellerAndValidator();
        uint256 orderId = _createOrder();

        vm.expectRevert();
        vm.prank(outsider);
        market.confirmAsSeller(orderId);

        _confirmSeller(orderId);

        vm.expectRevert();
        vm.prank(outsider);
        market.confirmAsValidator(orderId);

        _confirmValidator(orderId);

        vm.expectRevert();
        vm.prank(outsider);
        market.commitDelivery(orderId, deliveryHash);

        _commitDelivery(orderId);

        vm.expectRevert();
        vm.prank(outsider);
        market.acceptDelivery(orderId);

        vm.prank(buyer);
        market.openDispute(orderId);

        vm.expectRevert();
        vm.prank(outsider);
        market.resolveDispute(orderId, false, resolutionHash);
    }

    function _createCommittedOrder() private returns (uint256 orderId) {
        _registerSellerAndValidator();
        orderId = _createOrder();
        _confirmSeller(orderId);
        _confirmValidator(orderId);
        _commitDelivery(orderId);
    }

    function _registerSellerAndValidator() private {
        _registerSellerAndValidatorWithoutSupport();
        vm.prank(seller);
        market.addSupportedValidator(validator);
    }

    function _registerSellerAndValidatorWithoutSupport() private {
        vm.prank(seller);
        market.registerSeller("http://seller", PRICE, "http://content", contentHash, DELIVERY_TIMEOUT);

        vm.prank(validator);
        market.registerValidator("http://validator", VALIDATOR_FEE, RESPONSE_TIMEOUT);
    }

    function _createOrder() private returns (uint256 orderId) {
        vm.prank(buyer);
        orderId = market.createOrder{value: PRICE + VALIDATOR_FEE}(seller, validator, requestHash, APPROVAL_TIMEOUT);
    }

    function _confirmSeller(uint256 orderId) private {
        vm.prank(seller);
        market.confirmAsSeller(orderId);
    }

    function _confirmValidator(uint256 orderId) private {
        vm.prank(validator);
        market.confirmAsValidator(orderId);
    }

    function _commitDelivery(uint256 orderId) private {
        vm.prank(seller);
        market.commitDelivery(orderId, deliveryHash);
    }

    function _assertEq(uint256 got, uint256 want, string memory message) private pure {
        require(got == want, message);
    }

    function _assertEq(address got, address want, string memory message) private pure {
        require(got == want, message);
    }

    function _assertEq(bytes32 got, bytes32 want, string memory message) private pure {
        require(got == want, message);
    }
}
