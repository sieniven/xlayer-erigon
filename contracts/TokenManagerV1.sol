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
 * - Mint and Burn whitelists
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
    
    // Pagination and batch operation constants to prevent OOG
    uint256 public constant MAX_WHITELIST_RETURN = 100;
    uint256 public constant MAX_BATCH_SIZE = 20;
    
    // ==================== STATE VARIABLES ====================
    
    uint256 public activationBlock;
    
    // Whitelist storage - using mapping for O(1) access, array for enumeration with pagination
    mapping(address => bool) public mintWhitelist;
    mapping(address => bool) public burnWhitelist;
    address[] private _mintWhitelistArray;
    address[] private _burnWhitelistArray;
    
    // ==================== EVENTS ====================
    
    // System Events
    event Initialized(address indexed owner, uint256 activationBlock);
    event ActivationBlockSet(uint256 activationBlock);
    event ContractPaused(address indexed account);
    event ContractUnpaused(address indexed account);
    
    // Role Management Events
    event MinterRoleGranted(address indexed account, address indexed sender);
    event MinterRoleRevoked(address indexed account, address indexed sender);
    event BurnerRoleGranted(address indexed account, address indexed sender);
    event BurnerRoleRevoked(address indexed account, address indexed sender);
    
    // Whitelist Management Events
    event MintWhitelistAdded(address indexed account, address indexed sender);
    event MintWhitelistRemoved(address indexed account, address indexed sender);
    event BurnWhitelistAdded(address indexed account, address indexed sender);
    event BurnWhitelistRemoved(address indexed account, address indexed sender);
    
    // Token Operation Events
    event TokenMinted(address indexed to, uint256 amount, address indexed minter);
    event TokenBurned(address indexed from, uint256 amount, address indexed burner);
    event BatchTokenMinted(address[] indexed recipients, uint256[] amounts, address indexed minter);
    event BatchTokenBurned(address[] indexed sources, uint256[] amounts, address indexed burner);
    
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
     * @dev Modifier to check if address is in burn whitelist
     */
    modifier onlyBurnWhitelisted(address account) {
        require(isBurnAllowed(account), "Address is not in burn whitelist");
        _;
    }

    // ==================== INITIALIZATION ====================

    /**
     * @dev Initialize the contract (replaces constructor for upgradeable contracts)
     * @param _owner Initial owner and admin of the contract
     */
    function initialize(address _owner) external initializer {
        require(_owner != address(0), "Owner cannot be zero address");
        
        __Ownable_init(_owner);
        __AccessControl_init();
        __Pausable_init();
        __ReentrancyGuard_init();
        
        // Set up initial roles - owner is ONLY admin, NOT minter/burner by default
        _grantRole(ADMIN_ROLE, _owner);
        _setRoleAdmin(MINTER_ROLE, ADMIN_ROLE);
        _setRoleAdmin(BURNER_ROLE, ADMIN_ROLE);
        
        // NOTE: Owner does NOT get MINTER_ROLE or BURNER_ROLE by default
        // These must be explicitly granted if needed
        
        activationBlock = type(uint256).max; // Not active by default
        
        emit Initialized(_owner, activationBlock);
    }

    // ==================== PRECOMPILE FUNCTIONS ====================

    /**
     * @dev Check if precompile is available
     * @return bool True if precompile is available, false otherwise
     */
    function isPrecompileAvailable() public view returns (bool) {
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
    function setActivationBlock(uint256 _activationBlock) external onlyRole(ADMIN_ROLE) {
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
    function pause() external onlyRole(ADMIN_ROLE) {
        _pause();
        emit ContractPaused(_msgSender());
    }
    
    /**
     * @dev Unpause the contract
     */
    function unpause() external onlyRole(ADMIN_ROLE) {
        _unpause();
        emit ContractUnpaused(_msgSender());
    }

    // ==================== ROLE MANAGEMENT ====================

    /**
     * @dev Grant minter role to an account
     * @param account Address to grant minter role
     */
    function grantMinterRole(address account) external onlyRole(ADMIN_ROLE) {
        if (!hasRole(MINTER_ROLE, account)) {
            grantRole(MINTER_ROLE, account);
            emit MinterRoleGranted(account, _msgSender());
        }
    }

    /**
     * @dev Revoke minter role from an account
     * @param account Address to revoke minter role
     */
    function revokeMinterRole(address account) external onlyRole(ADMIN_ROLE) {
        if (hasRole(MINTER_ROLE, account)) {
            revokeRole(MINTER_ROLE, account);
            emit MinterRoleRevoked(account, _msgSender());
        }
    }

    /**
     * @dev Grant burner role to an account
     * @param account Address to grant burner role
     */
    function grantBurnerRole(address account) external onlyRole(ADMIN_ROLE) {
        if (!hasRole(BURNER_ROLE, account)) {
            grantRole(BURNER_ROLE, account);
            emit BurnerRoleGranted(account, _msgSender());
        }
    }

    /**
     * @dev Revoke burner role from an account
     * @param account Address to revoke burner role
     */
    function revokeBurnerRole(address account) external onlyRole(ADMIN_ROLE) {
        if (hasRole(BURNER_ROLE, account)) {
            revokeRole(BURNER_ROLE, account);
            emit BurnerRoleRevoked(account, _msgSender());
        }
    }

    // ==================== ROLE ENUMERATION (OWNER ONLY) ====================

    /**
     * @dev Get total number of minter role members (owner only)
     * @return uint256 Number of minter role members
     */
    function getMinterRoleCount() external view onlyOwner returns (uint256) {
        return getRoleMemberCount(MINTER_ROLE);
    }

    /**
     * @dev Get total number of burner role members (owner only)
     * @return uint256 Number of burner role members
     */
    function getBurnerRoleCount() external view onlyOwner returns (uint256) {
        return getRoleMemberCount(BURNER_ROLE);
    }

    /**
     * @dev Get all minter role members (owner only)
     * @return address[] Array of all minter addresses
     */
    function getAllMinters() external view onlyOwner returns (address[] memory) {
        return getRoleMembers(MINTER_ROLE);
    }

    /**
     * @dev Get all burner role members (owner only)
     * @return address[] Array of all burner addresses
     */
    function getAllBurners() external view onlyOwner returns (address[] memory) {
        return getRoleMembers(BURNER_ROLE);
    }

    /**
     * @dev Get paginated list of Minter role members (only owner can call)
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     */
    function getMintersPaginated(uint256 offset, uint256 limit) 
        external view onlyOwner returns (address[] memory) {
        return getRoleMembersPaginated(MINTER_ROLE, offset, limit);
    }

    /**
     * @dev Get paginated list of Burner role members (only owner can call)
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     */
    function getBurnersPaginated(uint256 offset, uint256 limit) 
        external view onlyOwner returns (address[] memory) {
        return getRoleMembersPaginated(BURNER_ROLE, offset, limit);
    }

    /**
     * @dev Internal function to get paginated role members
     */
    function getRoleMembersPaginated(bytes32 role, uint256 offset, uint256 limit) 
        internal view returns (address[] memory) {
        uint256 totalCount = getRoleMemberCount(role);
        
        if (offset >= totalCount) {
            return new address[](0);
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

    // ==================== MINT WHITELIST MANAGEMENT ====================

    /**
     * @dev Add address to mint whitelist
     * @param account Address to add to whitelist
     */
    function addMintWhitelist(address account) external onlyRole(ADMIN_ROLE) {
        require(!mintWhitelist[account], "Address is already in mint whitelist");
        
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
        
        mintWhitelist[account] = false;
        _removeFromArray(_mintWhitelistArray, account);
        emit MintWhitelistRemoved(account, _msgSender());
    }

    /**
     * @dev Get mint whitelist with pagination
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     * @return addresses Array of whitelisted addresses
     * @return total Total number of whitelisted addresses
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

    // ==================== BURN WHITELIST MANAGEMENT ====================

    /**
     * @dev Add address to burn whitelist
     * @param account Address to add to whitelist
     */
    function addBurnWhitelist(address account) external onlyRole(ADMIN_ROLE) {
        require(!burnWhitelist[account], "Address is already in burn whitelist");
        
        burnWhitelist[account] = true;
        _burnWhitelistArray.push(account);
        emit BurnWhitelistAdded(account, _msgSender());
    }
    
    /**
     * @dev Remove address from burn whitelist
     * @param account Address to remove from whitelist
     */
    function removeBurnWhitelist(address account) external onlyRole(ADMIN_ROLE) {
        require(burnWhitelist[account], "Address is not in burn whitelist");
        
        burnWhitelist[account] = false;
        _removeFromArray(_burnWhitelistArray, account);
        emit BurnWhitelistRemoved(account, _msgSender());
    }

    /**
     * @dev Get burn whitelist with pagination
     * @param offset Starting index
     * @param limit Maximum number of addresses to return
     * @return addresses Array of whitelisted addresses
     * @return total Total number of whitelisted addresses
     */
    function getBurnWhitelist(uint256 offset, uint256 limit) 
        external view 
        returns (address[] memory addresses, uint256 total) 
    {
        total = _burnWhitelistArray.length;
        
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
            addresses[i - offset] = _burnWhitelistArray[i];
        }
    }

    /**
     * @dev Get total count of burn whitelist addresses
     * @return uint256 Number of addresses in burn whitelist
     */
    function getBurnWhitelistCount() external view returns (uint256) {
        return _burnWhitelistArray.length;
    }

    /**
     * @dev Check if an address is allowed to have tokens burned from it
     * @param account Address to check
     * @return bool True if allowed, false otherwise
     */
    function isBurnAllowed(address account) public view returns (bool) {
        // If no whitelist entries, deny all (secure by default)
        if (_burnWhitelistArray.length == 0) {
            return false;
        }
        return burnWhitelist[account];
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
        require(from != address(0), "Cannot burn from zero address");
        require(amount > 0, "Amount must be greater than zero");
        
        // Protection: prevent burning entire balance to avoid potential issues
        uint256 currentBalance = from.balance;
        require(currentBalance > amount, "Cannot burn entire balance");
        
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
    
    /**
     * @dev Batch mint tokens to multiple addresses
     * @param recipients Array of addresses to mint tokens to
     * @param amounts Array of token amounts to mint
     */
    function batchMint(address[] calldata recipients, uint256[] calldata amounts) 
        external 
        onlyRole(MINTER_ROLE) 
        onlyActive 
        whenNotPaused 
        onlyWithPrecompile 
        nonReentrant
    {
        require(recipients.length == amounts.length, "Arrays length mismatch");
        require(recipients.length > 0, "Empty arrays");
        require(recipients.length <= MAX_BATCH_SIZE, "Too many recipients"); // Prevent OOG
        
        for (uint256 i = 0; i < recipients.length; i++) {
            address to = recipients[i];
            uint256 amount = amounts[i];
            
            require(to != address(0), "Cannot mint to zero address");
            require(amount > 0, "Amount must be greater than zero");
            require(isMintAllowed(to), "Address is not in mint whitelist");
            
            // Prepare precompile call data
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
        
        emit BatchTokenMinted(recipients, amounts, _msgSender());
    }
    
    /**
     * @dev Batch burn tokens from multiple addresses
     * @param sources Array of addresses to burn tokens from
     * @param amounts Array of token amounts to burn
     */
    function batchBurn(address[] calldata sources, uint256[] calldata amounts) 
        external 
        onlyRole(BURNER_ROLE) 
        onlyActive 
        whenNotPaused 
        onlyWithPrecompile 
        nonReentrant
    {
        require(sources.length == amounts.length, "Arrays length mismatch");
        require(sources.length > 0, "Empty arrays");
        require(sources.length <= MAX_BATCH_SIZE, "Too many sources"); // Prevent OOG
        
        for (uint256 i = 0; i < sources.length; i++) {
            address from = sources[i];
            uint256 amount = amounts[i];
            
            require(from != address(0), "Cannot burn from zero address");
            require(amount > 0, "Amount must be greater than zero");
            require(isBurnAllowed(from), "Address is not in burn whitelist");
            
            // Protection: prevent burning entire balance to avoid potential issues
            uint256 currentBalance = from.balance;
            require(currentBalance > amount, "Cannot burn entire balance");
            
            // Prepare precompile call data
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
        
        emit BatchTokenBurned(sources, amounts, _msgSender());
    }

    // ==================== BATCH OPERATIONS ====================

    /**
     * @dev Batch add addresses to mint whitelist
     * @param accounts Array of addresses to add to whitelist
     */
    function batchAddMintWhitelist(address[] calldata accounts) external onlyRole(ADMIN_ROLE) {
        require(accounts.length > 0, "Empty array");
        require(accounts.length <= MAX_BATCH_SIZE, "Too many addresses"); // Prevent OOG
        
        for (uint256 i = 0; i < accounts.length; i++) {
            address account = accounts[i];
            if (!mintWhitelist[account]) {
                mintWhitelist[account] = true;
                _mintWhitelistArray.push(account);
                emit MintWhitelistAdded(account, _msgSender());
            }
        }
    }

    /**
     * @dev Batch add addresses to burn whitelist
     * @param accounts Array of addresses to add to whitelist
     */
    function batchAddBurnWhitelist(address[] calldata accounts) external onlyRole(ADMIN_ROLE) {
        require(accounts.length > 0, "Empty array");
        require(accounts.length <= MAX_BATCH_SIZE, "Too many addresses"); // Prevent OOG
        
        for (uint256 i = 0; i < accounts.length; i++) {
            address account = accounts[i];
            if (!burnWhitelist[account]) {
                burnWhitelist[account] = true;
                _burnWhitelistArray.push(account);
                emit BurnWhitelistAdded(account, _msgSender());
            }
        }
    }

    // ==================== SECURITY OVERRIDES ====================

    /**
     * @dev Override transferOwnership to ensure proper role management
     * When ownership is transferred:
     * 1. Revoke ADMIN_ROLE from old owner
     * 2. Grant ADMIN_ROLE to new owner
     * 3. Transfer ownership
     * This ensures only one admin at any time
     */
    function transferOwnership(address newOwner) public virtual override onlyOwner {
        require(newOwner != address(0), "New owner cannot be zero address");
        
        address oldOwner = owner();
        
        // Revoke ADMIN_ROLE from old owner
        if (hasRole(ADMIN_ROLE, oldOwner)) {
            _revokeRole(ADMIN_ROLE, oldOwner);
        }
        
        // Grant ADMIN_ROLE to new owner
        _grantRole(ADMIN_ROLE, newOwner);
        
        // Transfer ownership using parent implementation
        super.transferOwnership(newOwner);
    }

    /**
     * @dev Override renounceOwnership to prevent accidental loss of control
     */
    function renounceOwnership() public virtual override {
        revert("TokenManager: renounceOwnership is disabled for security");
    }

    // ==================== UTILITY FUNCTIONS ====================

    /**
     * @dev Get current admin (same as owner for compatibility)
     * @return address Current owner address
     */
    function getAdmin() external view returns (address) {
        return owner();
    }
    
    /**
     * @dev Get contract version
     * @return string Contract version
     */
    function VERSION() external pure returns (string memory) {
        return "v1.0.0";
    }

    // ==================== INTERNAL FUNCTIONS ====================

    /**
     * @dev Internal helper function to remove an address from an array
     * @param array The array to remove from
     * @param item The address to remove
     */
    function _removeFromArray(address[] storage array, address item) internal {
        for (uint256 i = 0; i < array.length; i++) {
            if (array[i] == item) {
                // Move the last element to this position and remove the last element
                array[i] = array[array.length - 1];
                array.pop();
                break;
            }
        }
    }
} 