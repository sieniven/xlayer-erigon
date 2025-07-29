// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

/**
 * @title TokenManagerV1
 * @dev Token Manager implementation contract with mint/burn functionality
 * Uses OpenZeppelin standard contracts for security and upgradeability
 */
contract TokenManagerV1 is Initializable, OwnableUpgradeable, PausableUpgradeable {
    // Token Manager precompile address
    address constant PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000008888;
    
    // Operation codes for precompile
    bytes1 constant TEST_OP = 0x01;
    bytes1 constant MINT_OP = 0x02;
    bytes1 constant BURN_OP = 0x03;
    
    // State variables
    uint256 public activationBlock;
    mapping(address => bool) public burnWhitelist;
    address[] public burnWhitelistArray;
    
    // Events
    event ActivationBlockSet(uint256 activationBlock);
    event BurnWhitelistAdded(address indexed account);
    event BurnWhitelistRemoved(address indexed account);
    event TokenMinted(address indexed to, uint256 amount);
    event TokenBurned(address indexed from, uint256 amount);
    
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
     * @dev Check if precompile is available
     * @return bool True if precompile is available, false otherwise
     */
    function isPrecompileAvailable() public view returns (bool) {
        // Use the dedicated TEST_OP to check precompile availability
        // This operation doesn't require authentication and returns "OK" if successful
        bytes memory testData = abi.encodePacked(TEST_OP);
        
        (bool success, bytes memory returnData) = PRECOMPILE_ADDRESS.staticcall(testData);
        
        // If the call succeeded and returned "OK", precompile is available
        return success && returnData.length == 2 && 
               returnData[0] == 0x4F && returnData[1] == 0x4B; // "OK" in hex
    }
    
    /**
     * @dev Initialize the contract (replaces constructor for upgradeable contracts)
     * @param _owner Initial owner of the contract
     */
    function initialize(address _owner) external initializer {
        require(_owner != address(0), "Owner cannot be zero address");
        
        __Ownable_init(_owner);
        __Pausable_init();
        activationBlock = type(uint256).max; // Not active by default
        
        emit Initialized(_owner, activationBlock);
    }
    
    // Custom event for initialization (for compatibility)
    event Initialized(address indexed owner, uint256 activationBlock);
    
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
     * @dev Get current admin (same as owner for compatibility)
     * @return address Current owner address
     */
    function getAdmin() external view returns (address) {
        return owner();
    }
    
    /**
     * @dev Add address to burn whitelist
     * @param account Address to add to whitelist
     */
    function addBurnWhitelist(address account) external onlyOwner {
        require(!burnWhitelist[account], "Address is already whitelisted");
        
        burnWhitelist[account] = true;
        burnWhitelistArray.push(account);
        emit BurnWhitelistAdded(account);
    }
    
    /**
     * @dev Remove address from burn whitelist
     * @param account Address to remove from whitelist
     */
    function removeBurnWhitelist(address account) external onlyOwner {
        require(burnWhitelist[account], "Address is not whitelisted");
        
        burnWhitelist[account] = false;
        
        // Remove from array
        for (uint256 i = 0; i < burnWhitelistArray.length; i++) {
            if (burnWhitelistArray[i] == account) {
                burnWhitelistArray[i] = burnWhitelistArray[burnWhitelistArray.length - 1];
                burnWhitelistArray.pop();
                break;
            }
        }
        
        emit BurnWhitelistRemoved(account);
    }
    
    /**
     * @dev Get all burn whitelist addresses
     * @return address[] Array of whitelisted addresses
     */
    function getBurnWhitelist() external view returns (address[] memory) {
        return burnWhitelistArray;
    }
    
    /**
     * @dev Get burn whitelist count
     * @return uint256 Number of whitelisted addresses
     */
    function getBurnWhitelistCount() external view returns (uint256) {
        return burnWhitelistArray.length;
    }
    
    /**
     * @dev Check if burning is allowed for address
     * Returns true if address is in whitelist, or if whitelist is empty (backward compatibility)
     * @param account Address to check
     * @return bool True if burn is allowed, false otherwise
     */
    function isBurnAllowed(address account) public view returns (bool) {
        // If no whitelist entries, allow all (for backward compatibility)
        if (burnWhitelistArray.length == 0) {
            return true;
        }
        return burnWhitelist[account];
    }
    
    /**
     * @dev Mint tokens to an address
     * @param to Address to mint tokens to
     * @param amount Amount of tokens to mint
     */
    function mint(address to, uint256 amount) external onlyOwner onlyActive whenNotPaused onlyWithPrecompile {
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
        
        emit TokenMinted(to, amount);
    }
    
    /**
     * @dev Burn tokens from an address
     * @param from Address to burn tokens from
     * @param amount Amount of tokens to burn
     */
    function burn(address from, uint256 amount) external onlyOwner onlyActive whenNotPaused onlyWithPrecompile {
        require(from != address(0), "Cannot burn from zero address");
        require(amount > 0, "Amount must be greater than zero");
        
        // Check burn whitelist
        require(isBurnAllowed(from), "Address is not in burn whitelist");
        
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
        
        emit TokenBurned(from, amount);
    }
    
    /**
     * @dev Batch mint tokens to multiple addresses
     * @param recipients Array of addresses to mint tokens to
     * @param amounts Array of token amounts to mint
     */
    function batchMint(address[] calldata recipients, uint256[] calldata amounts) 
        external 
        onlyOwner 
        onlyActive 
        whenNotPaused 
        onlyWithPrecompile 
    {
        require(recipients.length == amounts.length, "Arrays length mismatch");
        require(recipients.length > 0, "Empty arrays");
        
        for (uint256 i = 0; i < recipients.length; i++) {
            require(recipients[i] != address(0), "Cannot mint to zero address");
            require(amounts[i] > 0, "Amount must be greater than zero");
            
            // Prepare precompile call data
            bytes memory callData = abi.encodePacked(
                MINT_OP,
                bytes32(uint256(uint160(recipients[i]))),
                bytes32(amounts[i])
            );
            
            // Call precompile
            (bool success, ) = PRECOMPILE_ADDRESS.call(callData);
            require(success, "Precompile call failed");
            
            emit TokenMinted(recipients[i], amounts[i]);
        }
    }
    
    /**
     * @dev Batch add addresses to burn whitelist
     * @param accounts Array of addresses to add to whitelist
     */
    function batchAddBurnWhitelist(address[] calldata accounts) external onlyOwner {
        require(accounts.length > 0, "Empty array");
        
        for (uint256 i = 0; i < accounts.length; i++) {
            address account = accounts[i];
            if (!burnWhitelist[account]) {
                burnWhitelist[account] = true;
                burnWhitelistArray.push(account);
                emit BurnWhitelistAdded(account);
            }
        }
    }
    
    /**
     * @dev Pause the contract (emergency stop)
     */
    function pause() external onlyOwner {
        _pause();
    }
    
    /**
     * @dev Unpause the contract
     */
    function unpause() external onlyOwner {
        _unpause();
    }
    
    /**
     * @dev Get contract version
     * @return string Contract version
     */
    function VERSION() external pure returns (string memory) {
        return "v1.0.0";
    }
} 