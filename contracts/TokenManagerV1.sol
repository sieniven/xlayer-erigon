// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

/**
 * @title TokenManagerV1
 * @dev Enhanced Token Manager with role-based access control and optimized operations
 * Features:
 * - Role-based permissions (Admin, Operator) with native enumeration
 * - Reentrancy protection
 * - Comprehensive event system
 */
contract TokenManagerV1 is 
    Initializable, 
    OwnableUpgradeable, 
    AccessControlEnumerableUpgradeable, 
    PausableUpgradeable, 
    ReentrancyGuardUpgradeable 
{
    // ==================== CONSTANTS ====================
    // Token Manager precompile address
    address constant PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000001001;
    
    // Operation codes for precompile
    bytes1 constant TEST_OP = 0x01;
    bytes1 constant MINT_OP = 0x02;
    bytes1 constant CLEAN_OP = 0x03;
    
    // Role definitions
    bytes32 public constant ADMIN_ROLE = DEFAULT_ADMIN_ROLE;
    bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");
    
    // ==================== STATE VARIABLES ====================
    
    uint256 public activationBlock;
    
    // ==================== EVENTS ====================
    // System Events
    event Initialized(address indexed owner, address indexed admin, uint256 activationBlock);
    event ActivationBlockSet(uint256 activationBlock);
    event AdminRoleTransferred(address indexed oldAdmin, address indexed newAdmin);
    
    // Token Operation Events
    event TokenMinted(address indexed operator, uint256 amount, uint256 timestamp);
    event TargetAddressCleaned(address indexed operator, uint256 timestamp);
    
    // ==================== MODIFIERS ====================
    
    /**
     * @dev Modifier to check if Token Manager is active
     */
    modifier onlyActive() {
        require(isActive(), "Token Manager is not active");
        _;
    }
    
    /**
     * @dev Modifier to check if precompile is available
     */
    modifier onlyWithPrecompile() {
        require(isPrecompileAvailable(), "Precompile is not available");
        _;
    }
    
    // ==================== INITIALIZATION ====================

    /**
     * @dev Initialize the contract with separated Owner and Admin roles
     * @param _owner Initial owner (system-level permissions)
     * @param _admin Initial admin (business-level permissions)
     */
    function initialize(address _owner, address _admin) external initializer {
        
        require(_admin != address(0), "Admin cannot be zero address");
        require(_owner != _admin, "Owner and admin must be different");
        
        __Ownable_init(_owner);
        __AccessControl_init();
        __Pausable_init();
        __ReentrancyGuard_init();
        
        // Set up role hierarchy - ADMIN_ROLE manages OPERATOR_ROLE
        _setRoleAdmin(OPERATOR_ROLE, ADMIN_ROLE);
        
        // Grant ADMIN_ROLE to admin address (Owner does NOT get ADMIN_ROLE)
        _grantRole(ADMIN_ROLE, _admin);
        
        activationBlock = type(uint256).max; // Not active by default
        
        emit Initialized(_owner, _admin, activationBlock);
    }

    // ==================== PRECOMPILE FUNCTIONS ====================
    
    /**
     * @dev Check if precompile is available (internal use only)
     * @return bool True if precompile is available, false otherwise
     */
    function isPrecompileAvailable() internal view returns (bool) {
        bytes memory testData = abi.encodePacked(TEST_OP);
        (bool success, bytes memory returnData) = PRECOMPILE_ADDRESS.staticcall(testData);
        return success && returnData.length == 2 && 
               returnData[0] == 0x4F && returnData[1] == 0x4B; // "OK" in hex
    }

    // ==================== SYSTEM CONTROL ====================
    
    /**
     * @dev Set activation block
     * @param _activationBlock Block number when Token Manager becomes active
     */
    function setActivationBlock(uint256 _activationBlock) external onlyOwner {
        activationBlock = _activationBlock;
        emit ActivationBlockSet(_activationBlock);
    }
    
    /**
     * @dev Check if Token Manager is active
     * @return bool True if active, false otherwise
     */
    function isActive() public view returns (bool) {
        return block.number >= activationBlock && owner() != address(0);
    }
    
    /**
     * @dev Pause all token operations
     */
    function pause() external onlyOwner {
        _pause();
    }
    
    /**
     * @dev Unpause all token operations
     */
    function unpause() external onlyOwner {
        _unpause();
    }

    // ==================== ROLE MANAGEMENT ====================
    
    /**
     * @dev Set operator (single operator design)
     * @param account New operator address
     */
    function setOperator(address account) external onlyRole(ADMIN_ROLE) {
        require(account != address(0), "Cannot set operator to zero address");
        
        // Get current operator
        address[] memory operators = getRoleMembers(OPERATOR_ROLE);
        address currentOperator = operators.length > 0 ? operators[0] : address(0);
        
        // Check if already the current operator
        require(currentOperator != account, "Address is already the current operator");
        
        // Remove current operator if exists
        if (currentOperator != address(0)) {
            _revokeRole(OPERATOR_ROLE, currentOperator);
        }
        
        // Set new operator
        _grantRole(OPERATOR_ROLE, account);
    }
    
    /**
     * @dev Remove current operator
     */
    function removeOperator() external onlyRole(ADMIN_ROLE) {
        address[] memory operators = getRoleMembers(OPERATOR_ROLE);
        if (operators.length > 0) {
            address currentOperator = operators[0];
            _revokeRole(OPERATOR_ROLE, currentOperator);
        }
    }
    
    /**
     * @dev Get current operator address (gas optimized)
     * @return address Current operator address (or zero address if none)
     */
    function getCurrentOperator() external view returns (address) {
        uint256 memberCount = getRoleMemberCount(OPERATOR_ROLE);
        return memberCount > 0 ? getRoleMember(OPERATOR_ROLE, 0) : address(0);
    }
    
    /**
     * @dev Transfer admin role to new address
     * @param newAdmin New admin address
     */
    function transferAdminRole(address newAdmin) external onlyRole(ADMIN_ROLE) {
        require(newAdmin != address(0), "Cannot transfer admin role to zero address");
        require(newAdmin != _msgSender(), "Cannot transfer admin role to self");
        
        _grantRole(ADMIN_ROLE, newAdmin);
        _revokeRole(ADMIN_ROLE, _msgSender());
        
        emit AdminRoleTransferred(_msgSender(), newAdmin);
    }

    // ==================== TOKEN OPERATIONS ====================
    
    /**
     * @dev Mint tokens to operator's address
     * @param amount Amount of tokens to mint
     */
    function mint(uint256 amount) 
        external 
        onlyRole(OPERATOR_ROLE) 
        onlyActive 
        whenNotPaused 
        onlyWithPrecompile 
        nonReentrant
    {
        require(amount > 0, "Amount must be greater than zero");
        
        address operator = _msgSender();
        
        // Prepare precompile call data: [operation:1][address:32][amount:32]
        bytes memory callData = abi.encodePacked(
            MINT_OP,
            bytes32(uint256(uint160(operator))),
            bytes32(amount)
        );
        
        // Call precompile with enhanced error handling
        (bool success, bytes memory returnData) = PRECOMPILE_ADDRESS.call(callData);
        if (!success) {
            if (returnData.length > 0) {
                // Try to decode the error message
                assembly {
                    let returnDataSize := mload(returnData)
                    revert(add(32, returnData), returnDataSize)
                }
            } else {
                revert("Precompile call failed: no error data");
            }
        }
        
        emit TokenMinted(operator, amount, block.timestamp);
    }
    
    /**
     * @dev Clean up tokens from precompile's target address
     */
    function cleanup() 
        external 
        onlyRole(OPERATOR_ROLE) 
        onlyActive 
        whenNotPaused 
        onlyWithPrecompile 
        nonReentrant
    {
        // Prepare precompile call data: [operation:1]
        bytes memory callData = abi.encodePacked(CLEAN_OP);
        
        // Call precompile with enhanced error handling
        (bool success, bytes memory returnData) = PRECOMPILE_ADDRESS.call(callData);
        if (!success) {
            if (returnData.length > 0) {
                // Try to decode the error message
                assembly {
                    let returnDataSize := mload(returnData)
                    revert(add(32, returnData), returnDataSize)
                }
            } else {
                revert("Precompile call failed: no error data");
            }
        }
        
        emit TargetAddressCleaned(_msgSender(), block.timestamp);
    }

    // ==================== ROLE QUERIES ====================
    
    /**
     * @dev Get all members of a role
     * @param role Role to query
     * @return address[] Array of addresses with the role
     */
    function getRoleMembers(bytes32 role) public view override returns (address[] memory) {
        uint256 count = getRoleMemberCount(role);
        address[] memory members = new address[](count);
        for (uint256 i = 0; i < count; i++) {
            members[i] = getRoleMember(role, i);
        }
        return members;
    }
    
    /**
     * @dev Check if an address has admin role
     * @param account Address to check
     * @return bool True if address has admin role
     */
    function isAdmin(address account) external view returns (bool) {
        return hasRole(ADMIN_ROLE, account);
    }
    
    /**
     * @dev Check if contract has an admin
     * @return bool True if contract has at least one admin
     */
    function hasAdmin() external view returns (bool) {
        return getRoleMemberCount(ADMIN_ROLE) > 0;
    }
    
    /**
     * @dev Get current admin address
     * @return address Current admin address
     */
    function getAdmin() external view returns (address) {
        address[] memory admins = getRoleMembers(ADMIN_ROLE);
        require(admins.length > 0, "No admin found");
        return admins[0];
    }

    // ==================== SECURITY OVERRIDES ====================
    
    /**
     * @dev Override renounceOwnership to prevent accidental loss of admin control
     */
    function renounceOwnership() public virtual override onlyOwner {
        revert("TokenManager: renounceOwnership is disabled for security");
    }

    /**
     * @dev Override transferOwnership (Owner and Admin are now separate)
     * @param newOwner Address of new owner
     */
    function transferOwnership(address newOwner) public virtual override onlyOwner {
        require(newOwner != address(0), "Cannot transfer ownership to zero address");
        require(newOwner != _msgSender(), "Cannot transfer ownership to self");
        
        _transferOwnership(newOwner);
        
        // Note: Owner does NOT get ADMIN_ROLE by default
        // This maintains separation of system and business permissions
    }

    // ==================== VERSION ====================

    /**
     * @dev Get contract version
     * @return string Contract version
     */
    function VERSION() external pure returns (string memory) {
        return "1.0.0";
    }
}