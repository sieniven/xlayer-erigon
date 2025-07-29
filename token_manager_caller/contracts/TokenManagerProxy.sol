// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

/**
 * @title TokenManagerProxy
 * @dev This contract implements a UUPS upgradeable proxy for TokenManagerCaller.
 * 
 * This proxy uses the ERC1967 standard for upgradeable proxies.
 * The implementation contract must inherit from UUPSUpgradeable to support upgrades.
 */
contract TokenManagerProxy is ERC1967Proxy {
    /**
     * @dev Initializes the upgradeable proxy with an initial implementation specified by `_implementation`.
     * 
     * If `_data` is nonempty, it's used as data in a delegate call to `_implementation`. This will typically be an
     * encoded function call, and allows initializing the storage of the proxy like a Solidity constructor.
     * 
     * @param _implementation Address of the initial implementation contract
     * @param _data Initialization data to be passed to the implementation's initialize function
     */
    constructor(
        address _implementation,
        bytes memory _data
    ) ERC1967Proxy(_implementation, _data) {}
    
    /**
     * @dev Receive function to handle plain Ether transfers.
     */
    receive() external payable {}
    
    /**
     * @dev Returns the current implementation address.
     * 
     * TIP: To get this value clients can read directly from the storage slot shown below (specified by EIP1967) using the
     * https://eth.wiki/json-rpc/API#eth_getstorageat[`eth_getStorageAt`] RPC call.
     * `0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc`
     */
    function implementation() external view returns (address) {
        return _implementation();
    }
} 