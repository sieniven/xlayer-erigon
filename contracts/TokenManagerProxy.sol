// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title TokenManagerProxy
 * @dev Transparent proxy contract for TokenManagerConfig using EIP-1967
 * This allows for upgradeable Token Manager configuration without changing the precompile integration
 */
contract TokenManagerProxy {
    // EIP-1967 storage slots
    bytes32 private constant _IMPLEMENTATION_SLOT = 0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc;
    bytes32 private constant _ADMIN_SLOT = 0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103;
    
    event Upgraded(address indexed implementation);
    event AdminChanged(address previousAdmin, address newAdmin);
    
    modifier onlyAdmin() {
        require(msg.sender == _getAdmin(), "Only admin can call this function");
        _;
    }
    
    constructor(address _logic, address _admin, bytes memory _data) {
        _setImplementation(_logic);
        _setAdmin(_admin);
        
        if (_data.length > 0) {
            (bool success,) = _logic.delegatecall(_data);
            require(success, "Initialization failed");
        }
    }
    
    // === Admin Functions ===
    
    function upgradeTo(address newImplementation) external onlyAdmin {
        _setImplementation(newImplementation);
        emit Upgraded(newImplementation);
    }
    
    function upgradeToAndCall(address newImplementation, bytes calldata data) external onlyAdmin {
        _setImplementation(newImplementation);
        emit Upgraded(newImplementation);
        
        if (data.length > 0) {
            (bool success,) = newImplementation.delegatecall(data);
            require(success, "Upgrade call failed");
        }
    }
    
    function changeAdmin(address newAdmin) external onlyAdmin {
        require(newAdmin != address(0), "New admin cannot be zero address");
        address oldAdmin = _getAdmin();
        _setAdmin(newAdmin);
        emit AdminChanged(oldAdmin, newAdmin);
    }
    
    // === View Functions ===
    
    function implementation() external view returns (address) {
        return _getImplementation();
    }
    
    function admin() external view returns (address) {
        return _getAdmin();
    }
    
    // === Internal Functions ===
    
    function _getImplementation() internal view returns (address impl) {
        bytes32 slot = _IMPLEMENTATION_SLOT;
        assembly {
            impl := sload(slot)
        }
    }
    
    function _setImplementation(address newImplementation) internal {
        require(newImplementation != address(0), "Implementation cannot be zero address");
        require(_isContract(newImplementation), "Implementation must be a contract");
        
        bytes32 slot = _IMPLEMENTATION_SLOT;
        assembly {
            sstore(slot, newImplementation)
        }
    }
    
    function _getAdmin() internal view returns (address adm) {
        bytes32 slot = _ADMIN_SLOT;
        assembly {
            adm := sload(slot)
        }
    }
    
    function _setAdmin(address newAdmin) internal {
        bytes32 slot = _ADMIN_SLOT;
        assembly {
            sstore(slot, newAdmin)
        }
    }
    
    function _isContract(address account) internal view returns (bool) {
        uint256 size;
        assembly {
            size := extcodesize(account)
        }
        return size > 0;
    }
    
    // === Fallback Function ===
    
    fallback() external payable {
        _delegate();
    }
    
    receive() external payable {
        _delegate();
    }
    
    function _delegate() internal {
        address impl = _getImplementation();
        require(impl != address(0), "Implementation not set");
        
        assembly {
            calldatacopy(0, 0, calldatasize())
            let result := delegatecall(gas(), impl, 0, calldatasize(), 0, 0)
            returndatacopy(0, 0, returndatasize())
            
            switch result
            case 0 {
                revert(0, returndatasize())
            }
            default {
                return(0, returndatasize())
            }
        }
    }
} 