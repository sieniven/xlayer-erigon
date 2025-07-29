// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";

contract TokenManagerCallerUpgradeable is Initializable, OwnableUpgradeable, UUPSUpgradeable {
    address constant TOKEN_MANAGER_PRECOMPILE = 0x0000000000000000000000000000000000008888;
    
    // Mapping to store addresses that can perform mint/burn operations
    mapping(address => bool) public canOperate;
    
    // Mapping to store addresses that can be burned from (additional check for burn operations)
    mapping(address => bool) public canBeBurned;
    
    // Events for tracking permission changes
    event OperatePermissionGranted(address indexed account);
    event OperatePermissionRevoked(address indexed account);
    event BurnTargetPermissionGranted(address indexed account);
    event BurnTargetPermissionRevoked(address indexed account);
    
    // Events for contract lifecycle
    event ContractInitialized(address indexed owner);
    event ContractUpgraded(address indexed newImplementation);
    
    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }
    
    function initialize(address initialOwner) public initializer {
        __Ownable_init(initialOwner);
        __UUPSUpgradeable_init();
        
        emit ContractInitialized(initialOwner);
    }
    
    function _authorizeUpgrade(address newImplementation) internal onlyOwner override {
        emit ContractUpgraded(newImplementation);
    }
    
    // Owner-only functions to manage operation permissions (mint & burn)
    function grantOperatePermission(address account) external onlyOwner {
        canOperate[account] = true;
        emit OperatePermissionGranted(account);
    }
    
    function revokeOperatePermission(address account) external onlyOwner {
        canOperate[account] = false;
        emit OperatePermissionRevoked(account);
    }
    
    // Owner-only functions to manage burn target permissions
    function grantBurnTargetPermission(address account) external onlyOwner {
        canBeBurned[account] = true;
        emit BurnTargetPermissionGranted(account);
    }
    
    function revokeBurnTargetPermission(address account) external onlyOwner {
        canBeBurned[account] = false;
        emit BurnTargetPermissionRevoked(account);
    }
    
    
    // Public mint function with operation permission check
    function mint(address to, uint256 amount) external returns (uint256) {
        require(canOperate[msg.sender], "Caller not authorized to operate");
        return _mint(to, amount);
    }
    
    // Public burn function with dual permission checks
    function burn(address from, uint256 amount) external returns (uint256) {
        require(canOperate[msg.sender], "Caller not authorized to operate");
        require(canBeBurned[from], "Target address not authorized to be burned from");
        return _burn(from, amount);
    }
    
    function _mint(address to, uint256 amount) internal returns (uint256) {
        // Directly concatenate parameters: 1-byte op + 20-byte address + 32-byte amount
        bytes memory input = abi.encodePacked(bytes1(0x01), to, amount);
        
        (bool success, ) = TOKEN_MANAGER_PRECOMPILE.call(input);
        require(success, "Mint token failed");
                 
        return amount;
    }

    function _burn(address from, uint256 amount) internal returns (uint256) {
        // Directly concatenate parameters: 1-byte op + 20-byte address + 32-byte amount
        bytes memory input = abi.encodePacked(bytes1(0x02), from, amount);
        
        (bool success, ) = TOKEN_MANAGER_PRECOMPILE.call(input);
        require(success, "Burn token failed");
        
        return amount;
    }
    
    // Version function for upgrade tracking
    function getVersion() public pure returns (string memory) {
        return "1.0.0";
    }
} 