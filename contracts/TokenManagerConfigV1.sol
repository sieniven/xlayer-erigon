// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title TokenManagerConfigV1
 * @dev Implementation contract for Token Manager configuration (Version 1)
 * This contract is designed to work with a proxy for upgradeability
 */
contract TokenManagerConfigV1 {
    // === Storage Layout (V1) ===
    // IMPORTANT: Never change the order or remove existing storage variables in upgrades
    
    address public owner;                    // Slot 0: Contract owner (also serves as admin)
    uint256 public activationBlock;         // Slot 1: Token Manager activation block
    address[] public burnWhitelist;         // Slot 2: Burn whitelist addresses (dynamic array)
    mapping(address => bool) public isBurnWhitelisted; // Slot 3: Burn whitelist mapping
    
    // === Version Information ===
    string public constant VERSION = "1.0.0";
    uint256 public constant VERSION_NUMBER = 1;
    
    // === Events ===
    event OwnerChanged(address indexed oldOwner, address indexed newOwner);
    event ActivationBlockSet(uint256 blockNumber);
    event BurnWhitelistAdded(address indexed addr);
    event BurnWhitelistRemoved(address indexed addr);
    event Initialized(address indexed owner, uint256 activationBlock);
    
    // === Modifiers ===
    modifier onlyOwner() {
        require(msg.sender == owner, "Only owner can call this function");
        _;
    }
    
    modifier notInitialized() {
        require(owner == address(0), "Already initialized");
        _;
    }
    
    /// @dev Initialize the contract (replaces constructor for proxy pattern)
    function initialize(address _initialOwner) external notInitialized {
        require(_initialOwner != address(0), "Owner cannot be zero address");
        
        owner = _initialOwner;
        activationBlock = type(uint256).max; // Default: not activated
        
        emit Initialized(_initialOwner, activationBlock);
    }
    
    // === Owner Management ===
    
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "New owner cannot be zero address");
        emit OwnerChanged(owner, newOwner);
        owner = newOwner;
    }
    
    // === Activation Management ===
    
    function setActivationBlock(uint256 _activationBlock) external onlyOwner {
        activationBlock = _activationBlock;
        emit ActivationBlockSet(_activationBlock);
    }
    
    function isActive() external view returns (bool) {
        return block.number >= activationBlock && owner != address(0);
    }
    
    // === Admin Management (owner is admin in V1) ===
    
    function getAdmin() external view returns (address) {
        return owner; // Owner is the admin in V1
    }
    
    // === Burn Whitelist Management ===
    
    function addBurnWhitelist(address addr) external onlyOwner {
        require(addr != address(0), "Address cannot be zero address");
        require(!isBurnWhitelisted[addr], "Address is already whitelisted");
        
        burnWhitelist.push(addr);
        isBurnWhitelisted[addr] = true;
        emit BurnWhitelistAdded(addr);
    }
    
    function removeBurnWhitelist(address addr) external onlyOwner {
        require(isBurnWhitelisted[addr], "Address is not whitelisted");
        
        isBurnWhitelisted[addr] = false;
        
        // Remove from array
        for (uint256 i = 0; i < burnWhitelist.length; i++) {
            if (burnWhitelist[i] == addr) {
                burnWhitelist[i] = burnWhitelist[burnWhitelist.length - 1];
                burnWhitelist.pop();
                break;
            }
        }
        
        emit BurnWhitelistRemoved(addr);
    }
    
    function getBurnWhitelist() external view returns (address[] memory) {
        return burnWhitelist;
    }
    
    function getBurnWhitelistCount() external view returns (uint256) {
        return burnWhitelist.length;
    }
    
    function isBurnAllowed(address addr) external view returns (bool) {
        // If no whitelist configured, allow all addresses
        if (burnWhitelist.length == 0) {
            return true;
        }
        return isBurnWhitelisted[addr];
    }
    
    // === Batch Operations ===
    
    function batchAddBurnWhitelist(address[] calldata addresses) external onlyOwner {
        for (uint256 i = 0; i < addresses.length; i++) {
            if (addresses[i] != address(0) && !isBurnWhitelisted[addresses[i]]) {
                burnWhitelist.push(addresses[i]);
                isBurnWhitelisted[addresses[i]] = true;
                emit BurnWhitelistAdded(addresses[i]);
            }
        }
    }
    
    // === Upgrade Support Functions ===
    
    function getStorageLayout() external pure returns (string memory) {
        return "V1: owner(0), activationBlock(1), burnWhitelist(2), isBurnWhitelisted(3)";
    }
} 