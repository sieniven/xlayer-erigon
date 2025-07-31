// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/extensions/AccessControlEnumerableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/ReentrancyGuardUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

/**
 * @title TokenManagerV1
 * @dev Enhanced Token Manager with role-based access control and optimized whitelist management
 * Features:
 * - Role-based permissions (Admin, Minter, Burner) with native enumeration
 * - Dynamic mint whitelist management
 * - Hard-coded burn whitelist for security
 * - Reentrancy protection
 * - Optimized array operations with pagination
 * - Comprehensive event system
 * - Native role member enumeration (owner-only access)
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
    address constant PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000008888;
    
    // Operation codes for precompile
    bytes1 constant TEST_OP = 0x01;
    bytes1 constant MINT_OP = 0x02;
    bytes1 constant BURN_OP = 0x03;
    
    // Role definitions
    bytes32 public constant ADMIN_ROLE = DEFAULT_ADMIN_ROLE;
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    bytes32 public constant BURNER_ROLE = keccak256("BURNER_ROLE");
    
    // Pagination constant to prevent OOG
    uint256 public constant MAX_WHITELIST_RETURN = 100;
    uint256 public constant MAX_WHITELIST_SIZE = 500; // Maximum number of addresses in mint whitelist
    
    // ==================== STATE VARIABLES ====================
    
    uint256 public activationBlock;
    
    // Dynamic mint whitelist storage
    mapping(address => bool) public mintWhitelist;
    address[] private _mintWhitelistArray;
    
    // Hard-coded burn whitelist addresses (immutable for security)
    address[] private _hardCodedBurnAddresses;
    
    // ==================== EVENTS ====================
    
    // System Events
    event Initialized(address indexed owner, address indexed admin, uint256 activationBlock);
    event ActivationBlockSet(uint256 activationBlock);
    event ContractPaused(address indexed account);
    event ContractUnpaused(address indexed account);
    event AdminRoleTransferred(address indexed oldAdmin, address indexed newAdmin);
    
    // Role Management Events
    event MinterRoleGranted(address indexed account, address indexed sender);
    event MinterRoleRevoked(address indexed account, address indexed sender);
    event BurnerRoleGranted(address indexed account, address indexed sender);
    event BurnerRoleRevoked(address indexed account, address indexed sender);
    
    // Whitelist Management Events (only for mint whitelist)
    event MintWhitelistAdded(address indexed account, address indexed sender);
    event MintWhitelistRemoved(address indexed account, address indexed sender);
    
    // Token Operation Events
    event TokenMinted(address indexed to, uint256 amount, address indexed minter);
    event TokenBurned(address indexed from, uint256 amount, address indexed burner);
    
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
    
    /**
     * @dev Modifier to check if address is in mint whitelist
     */
    modifier onlyMintWhitelisted(address account) {
        require(isMintAllowed(account), "Address is not in mint whitelist");
        _;
    }
    
    /**
     * @dev Modifier to check if address is in hard-coded burn whitelist
     */
    modifier onlyBurnWhitelisted(address account) {
        require(isBurnAllowed(account), "Address is not in burn whitelist");
        _;
    }

    // ==================== INITIALIZATION ====================

    /**
     * @dev Initialize the contract with separated Owner and Admin roles
     * @param _owner Initial owner (system-level permissions)
     * @param _admin Initial admin (business-level permissions)
     */
    function initialize(address _owner, address _admin) external initializer {
        require(_owner != address(0), "Owner cannot be zero address");
        require(_admin != address(0), "Admin cannot be zero address");
        require(_owner != _admin, "Owner and admin must be different");
        
        __Ownable_init(_owner);
        __AccessControl_init();
        __Pausable_init();
        __ReentrancyGuard_init();
        
        // Set up role hierarchy - ADMIN_ROLE manages MINTER and BURNER roles
        _setRoleAdmin(MINTER_ROLE, ADMIN_ROLE);
        _setRoleAdmin(BURNER_ROLE, ADMIN_ROLE);
        
        // Grant ADMIN_ROLE to admin address (Owner does NOT get ADMIN_ROLE)
        _grantRole(ADMIN_ROLE, _admin);
        
        // NOTE: Owner does NOT get ADMIN_ROLE, MINTER_ROLE or BURNER_ROLE by default
        // Owner focuses on system-level operations, Admin handles business operations
        
        // Initialize hard-coded burn addresses (immutable for security)
        _initializeBurnAddresses();
        
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
     * @dev Pause the contract (emergency stop)
     */
    function pause() external onlyOwner {
        _pause();
        emit ContractPaused(_msgSender());
    }
    
    /**
     * @dev Unpause the contract
     */
    function unpause() external onlyOwner {
        _unpause();
        emit ContractUnpaused(_msgSender());
    }

    // ==================== ROLE MANAGEMENT ====================

    /**
     * @dev Grant minter role to an account (only admin can call)
     * @param account Address to grant role to
     */
    function grantMinterRole(address account) external onlyRole(ADMIN_ROLE) {
        if (!hasRole(MINTER_ROLE, account)) {
            _grantRole(MINTER_ROLE, account);
            emit MinterRoleGranted(account, _msgSender());
        }
    }

    /**
     * @dev Revoke minter role from an account (only admin can call)
     * @param account Address to revoke role from
     */
    function revokeMinterRole(address account) external onlyRole(ADMIN_ROLE) {
        if (hasRole(MINTER_ROLE, account)) {
            _revokeRole(MINTER_ROLE, account);
            emit MinterRoleRevoked(account, _msgSender());
        }
    }

    /**
     * @dev Grant burner role to an account (only admin can call)
     * @param account Address to grant role to
     */
    function grantBurnerRole(address account) external onlyRole(ADMIN_ROLE) {
        if (!hasRole(BURNER_ROLE, account)) {
            _grantRole(BURNER_ROLE, account);
            emit BurnerRoleGranted(account, _msgSender());
        }
    }

    /**
     * @dev Revoke burner role from an account (only admin can call)
     * @param account Address to revoke role from
     */
    function revokeBurnerRole(address account) external onlyRole(ADMIN_ROLE) {
        if (hasRole(BURNER_ROLE, account)) {
            _revokeRole(BURNER_ROLE, account);
            emit BurnerRoleRevoked(account, _msgSender());
        }
    }

    // ==================== ADMIN ROLE MANAGEMENT ====================

    /**
     * @dev Get current admin address
     * @return address Current admin address
     */
    function getAdmin() external view returns (address) {
        uint256 adminCount = getRoleMemberCount(ADMIN_ROLE);
        require(adminCount > 0, "No admin assigned");
        return getRoleMember(ADMIN_ROLE, 0);
    }

    /**
     * @dev Check if address is admin
     * @param account Address to check
     * @return bool True if account is admin
     */
    function isAdmin(address account) external view returns (bool) {
        return hasRole(ADMIN_ROLE, account);
    }
    
    /**
     * @dev Check if admin role is assigned
     * @return bool True if there is an admin
     */
    function hasAdmin() external view returns (bool) {
        return getRoleMemberCount(ADMIN_ROLE) > 0;
    }

    /**
     * @dev Transfer admin role to new address (only current admin can call)
     * @param newAdmin New admin address
     */
    function transferAdminRole(address newAdmin) external onlyRole(ADMIN_ROLE) {
        require(newAdmin != address(0), "New admin cannot be zero address");
        require(newAdmin != _msgSender(), "Cannot transfer to self");
        require(!hasRole(ADMIN_ROLE, newAdmin), "Address already has admin role");
        
        address oldAdmin = _msgSender();
        
        // Revoke admin role from current admin
        _revokeRole(ADMIN_ROLE, oldAdmin);
        
        // Grant admin role to new admin
        _grantRole(ADMIN_ROLE, newAdmin);
        
        emit AdminRoleTransferred(oldAdmin, newAdmin);
    }

    // ==================== ROLE ENUMERATION (ADMIN ONLY) ====================

    /**
     * @dev Get number of accounts with minter role
     * @return uint256 Number of minter role accounts
     */
    function getMinterRoleCount() external view returns (uint256) {
        return getRoleMemberCount(MINTER_ROLE);
    }

    /**
     * @dev Get number of accounts with burner role
     * @return uint256 Number of burner role accounts
     */
    function getBurnerRoleCount() external view returns (uint256) {
        return getRoleMemberCount(BURNER_ROLE);
    }

    /**
     * @dev Get paginated list of Minter role members (only admin can call)
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     */
    function getMintersPaginated(uint256 offset, uint256 limit) 
        external view onlyRole(ADMIN_ROLE) returns (address[] memory) {
        return getRoleMembersPaginated(MINTER_ROLE, offset, limit);
    }

    /**
     * @dev Get paginated list of Burner role members (only admin can call)
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     */
    function getBurnersPaginated(uint256 offset, uint256 limit) 
        external view onlyRole(ADMIN_ROLE) returns (address[] memory) {
        return getRoleMembersPaginated(BURNER_ROLE, offset, limit);
    }

    // ==================== MINT WHITELIST MANAGEMENT ====================

    /**
     * @dev Add address to mint whitelist
     * @param account Address to add to whitelist
     */
    function addMintWhitelist(address account) external onlyRole(ADMIN_ROLE) {
        require(!mintWhitelist[account], "Address is already in mint whitelist");
        require(_mintWhitelistArray.length < MAX_WHITELIST_SIZE, "Whitelist size limit reached");
        
        mintWhitelist[account] = true;
        _mintWhitelistArray.push(account);
        emit MintWhitelistAdded(account, _msgSender());
    }
    
    /**
     * @dev Remove address from mint whitelist
     * @param account Address to remove from whitelist
     */
    function removeMintWhitelist(address account) external onlyRole(ADMIN_ROLE) {
        require(mintWhitelist[account], "Address is not in mint whitelist");
        
        // Check array operation success first
        uint256 originalLength = _mintWhitelistArray.length;
        _removeFromArray(_mintWhitelistArray, account);
        
        // Verify array was actually modified
        require(_mintWhitelistArray.length < originalLength, "Failed to remove from array");
        
        // Update mapping last
        mintWhitelist[account] = false;
        emit MintWhitelistRemoved(account, _msgSender());
    }

    /**
     * @dev Get mint whitelist with pagination
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     * @return addresses Array of whitelisted addresses
     * @return total Total number of addresses in whitelist
     */
    function getMintWhitelist(uint256 offset, uint256 limit) 
        external view 
        returns (address[] memory addresses, uint256 total) 
    {
        total = _mintWhitelistArray.length;
        
        if (offset >= total) {
            return (new address[](0), total);
        }
        
        // Cap the limit to prevent OOG
        if (limit > MAX_WHITELIST_RETURN) {
            limit = MAX_WHITELIST_RETURN;
        }
        
        uint256 end = offset + limit;
        if (end > total) {
            end = total;
        }
        
        addresses = new address[](end - offset);
        for (uint256 i = offset; i < end; i++) {
            addresses[i - offset] = _mintWhitelistArray[i];
        }
    }

    /**
     * @dev Get total count of mint whitelist addresses
     * @return uint256 Number of addresses in mint whitelist
     */
    function getMintWhitelistCount() external view returns (uint256) {
        return _mintWhitelistArray.length;
    }

    /**
     * @dev Check if an address is allowed to receive minted tokens
     * @param account Address to check
     * @return bool True if allowed, false otherwise
     */
    function isMintAllowed(address account) public view returns (bool) {
        // If no whitelist entries, deny all (secure by default)
        if (_mintWhitelistArray.length == 0) {
            return false;
        }
        return mintWhitelist[account];
    }

    // ==================== BURN WHITELIST QUERIES ====================
    
    /**
     * @dev Get hard-coded burn whitelist addresses
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     * @return addresses Array of burn-allowed addresses
     * @return total Total number of burn-allowed addresses
     */
    function getBurnWhitelist(uint256 offset, uint256 limit) 
        external view 
        returns (address[] memory addresses, uint256 total) 
    {
        total = _hardCodedBurnAddresses.length;
        
        if (offset >= total) {
            return (new address[](0), total);
        }
        
        // Cap the limit to prevent OOG
        if (limit > MAX_WHITELIST_RETURN) {
            limit = MAX_WHITELIST_RETURN;
        }
        
        uint256 end = offset + limit;
        if (end > total) {
            end = total;
        }
        
        addresses = new address[](end - offset);
        for (uint256 i = offset; i < end; i++) {
            addresses[i - offset] = _hardCodedBurnAddresses[i];
        }
    }
    
    /**
     * @dev Get total count of burn whitelist addresses
     * @return uint256 Number of addresses in burn whitelist
     */
    function getBurnWhitelistCount() external view returns (uint256) {
        return _hardCodedBurnAddresses.length;
    }
    
    /**
     * @dev Check if an address is allowed to have tokens burned from it (hard-coded addresses only)
     * @param account Address to check
     * @return bool True if allowed, false otherwise
     */
    function isBurnAllowed(address account) public view returns (bool) {
        // Check against hard-coded burn addresses
        for (uint256 i = 0; i < _hardCodedBurnAddresses.length; i++) {
            if (_hardCodedBurnAddresses[i] == account) {
            return true;
            }
        }
        return false;
    }

    // ==================== TOKEN OPERATIONS ====================
    
    /**
     * @dev Mint tokens to an address
     * @param to Address to mint tokens to
     * @param amount Amount of tokens to mint
     */
    function mint(address to, uint256 amount) 
        external 
        onlyRole(MINTER_ROLE) 
        onlyActive 
        whenNotPaused 
        onlyWithPrecompile 
        onlyMintWhitelisted(to)
        nonReentrant
    {
        require(to != address(0), "Cannot mint to zero address");
        require(amount > 0, "Amount must be greater than zero");
        
        // Prepare precompile call data: [operation:1][address:32][amount:32]
        bytes memory callData = abi.encodePacked(
            MINT_OP,
            bytes32(uint256(uint160(to))),
            bytes32(amount)
        );
        
        // Call precompile
        (bool success, ) = PRECOMPILE_ADDRESS.call(callData);
        require(success, "Precompile call failed");
        
        emit TokenMinted(to, amount, _msgSender());
    }
    
    /**
     * @dev Burn tokens from an address
     * @param from Address to burn tokens from
     * @param amount Amount of tokens to burn
     */
    function burn(address from, uint256 amount) 
        external 
        onlyRole(BURNER_ROLE) 
        onlyActive 
        whenNotPaused 
        onlyWithPrecompile 
        onlyBurnWhitelisted(from)
        nonReentrant
    {
        // Note: Zero address burn restriction removed to allow burning from 0x0000... address
        // This is consistent with industry practice where zero address is used as a "black hole" for token destruction
        // Zero address is already included in the hardcoded burn whitelist, so this restriction was redundant
        // require(from != address(0), "Cannot burn from zero address");

        require(amount > 0, "Amount must be greater than zero");
        
        // Protection: prevent burning entire balance to avoid potential issues
        uint256 currentBalance = from.balance;
        require(currentBalance >= amount, "Insufficient balance for burn"); // Check if balance is enough
        require(currentBalance > amount, "Cannot burn entire balance, must leave at least 1 wei"); // Check if burning entire balance
        
        // Prepare precompile call data: [operation:1][address:32][amount:32]
        bytes memory callData = abi.encodePacked(
            BURN_OP,
            bytes32(uint256(uint160(from))),
            bytes32(amount)
        );
        
        // Call precompile
        (bool success, ) = PRECOMPILE_ADDRESS.call(callData);
        require(success, "Precompile call failed");
        
        emit TokenBurned(from, amount, _msgSender());
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
        require(newOwner != address(0), "New owner cannot be zero address");
        require(newOwner != owner(), "Cannot transfer to self");
            
        // Owner and Admin are separate - just transfer ownership
        // Admin role remains unchanged
        super.transferOwnership(newOwner);
    }

    // ==================== UTILITY FUNCTIONS ====================

    /**
     * @dev Get contract version
     * @return string Version string
     */
    function VERSION() external pure returns (string memory) {
        return "v1.0.0";
        }

    // ==================== INTERNAL FUNCTIONS ====================
    
    /**
     * @dev Initialize hard-coded burn addresses
     * This function sets the immutable burn whitelist
     */
    function _initializeBurnAddresses() internal {
        _hardCodedBurnAddresses.push(0x000000000000000000000000000000000000dEaD); // Burn address
        _hardCodedBurnAddresses.push(0x0000000000000000000000000000000000000000); // Zero address
    }

    /**
     * @dev Remove an address from an array (internal helper)
     * @param array The array to remove from
     * @param account The address to remove
     */
    function _removeFromArray(address[] storage array, address account) internal {
        for (uint256 i = 0; i < array.length; i++) {
            if (array[i] == account) {
                array[i] = array[array.length - 1];
                array.pop();
                break;
            }
        }
    }
    
    /**
     * @dev Get paginated role members (internal helper)
     * @param role The role to query
     * @param offset Starting index
     * @param limit Maximum number of results
     * @return address[] Array of role members
     */
    function getRoleMembersPaginated(bytes32 role, uint256 offset, uint256 limit) 
        internal view returns (address[] memory) {
        uint256 totalCount = getRoleMemberCount(role);
        
        if (offset >= totalCount) {
            return new address[](0);
        }
        
        // Cap the limit to prevent OOG
        if (limit > MAX_WHITELIST_RETURN) {
            limit = MAX_WHITELIST_RETURN;
        }
        
        uint256 length = limit;
        if (offset + limit > totalCount) {
            length = totalCount - offset;
        }
        
        address[] memory result = new address[](length);
        for (uint256 i = 0; i < length; i++) {
            result[i] = getRoleMember(role, offset + i);
        }
        
        return result;
    }
} 