//SPDX-License-Identifier: Unlicense
pragma solidity ^0.8.4;



contract WOKB {
        /**
         * @dev Indicates a failure with the token `receiver`. Used in transfers.
         * @param receiver Address to which tokens are being transferred.
         */
        error ERC20InvalidReceiver(address receiver);

        error ERC20IBurn(address acct);
        error ERC20TransferFrom(address acct);

        /**
             * @dev Indicates a failure with the token `sender`. Used in transfers.
             * @param sender Address whose tokens are being transferred.
             */
            error ERC20InvalidSender(address sender);

            /**
     * @dev Indicates an error related to the current `balance` of a `sender`. Used in transfers.
     * @param sender Address whose tokens are being transferred.
     * @param balance Current balance for the interacting account.
     * @param needed Minimum amount required to perform a transfer.
     */
    error ERC20InsufficientBalance(address sender, uint256 balance, uint256 needed);


    string public name     = "Wrapped OKB";
    string public symbol   = "WOKB";
    uint8  public decimals = 18;

    address private zeroAddress = 0x0000000000000000000000000000000000000000;

    event  Approval(address indexed src, address indexed guy, uint wad);
    event  Transfer(address indexed src, address indexed dst, uint wad);
    event  Deposit(address indexed dst, uint wad);
    event  Withdrawal(address indexed src, uint wad);

    mapping (address => uint)                       public  balanceOf;
    mapping (address => mapping (address => uint))  public  allowance;


 // PolygonZkEVM Bridge address
    address public bridgeAddress;
    modifier onlyBridge() {
        require(
            msg.sender == bridgeAddress,
            "TokenWrapped::onlyBridge: Not PolygonZkEVMBridge"
        );
        _;
    }

    constructor(address _bridge) {
        bridgeAddress=_bridge;
    }

   receive() external payable {
        deposit();
    }

    function deposit() public payable {
        balanceOf[msg.sender] += msg.value;
        emit Deposit(msg.sender, msg.value);
        emit Transfer(zeroAddress, msg.sender, msg.value);
    }
    // function withdraw(uint wad) public {
    //     require(balanceOf[msg.sender] >= wad);
    //     balanceOf[msg.sender] -= wad;
    //     msg.sender.transfer(wad);
    //     emit Withdrawal(msg.sender, wad);
    //     emit Transfer(msg.sender, zeroAddress, wad);
    // }

    function totalSupply() public view returns (uint) {
        return address(this).balance;
    }

    function approve(address guy, uint wad) public returns (bool) {
        allowance[msg.sender][guy] = wad;
        emit Approval(msg.sender, guy, wad);
        return true;
    }

    function transfer(address dst, uint wad) public returns (bool) {
        return transferFrom(msg.sender, dst, wad);
    }

    // function safeTransferFrom( address from, address to, uint256 value) public {
    //     return transferFrom(from, to, value);
    // }

    function transferFrom(address src, address dst, uint wad)
        public
        returns (bool)
    {
        // revert ERC20TransferFrom(src);
              require(
            false,
            "transferFrom error"
        );
        // require(balanceOf[src] >= wad);

        // if (src != msg.sender ) {
        //     require(allowance[src][msg.sender] >= wad);
        //     allowance[src][msg.sender] -= wad;
        // }

        // balanceOf[src] -= wad;
        // balanceOf[dst] += wad;

        // emit Transfer(src, dst, wad);

        return true;
    }

        function mint(address to, uint256 value) external onlyBridge {
        _mint(to, value);
    }

    // Notice that is not require to approve wrapped tokens to use the bridge
    function burn(address account, uint256 value) external onlyBridge {
        _burn(account, value);
    }

        /**
         * @dev Creates a `value` amount of tokens and assigns them to `account`, by transferring it from address(0).
         * Relies on the `_update` mechanism
         *
         * Emits a {Transfer} event with `from` set to the zero address.
         *
         * NOTE: This function is not virtual, {_update} should be overridden instead.
         */
        function _mint(address account, uint256 value) internal {
            if (account == address(0)) {
                revert ERC20InvalidReceiver(address(0));
            }
            _update(address(0), account, value);
        }

        /**
         * @dev Destroys a `value` amount of tokens from `account`, lowering the total supply.
         * Relies on the `_update` mechanism.
         *
         * Emits a {Transfer} event with `to` set to the zero address.
         *
         * NOTE: This function is not virtual, {_update} should be overridden instead
         */
        function _burn(address account, uint256 value) internal {
            if (account == address(0)) {
                // revert ERC20InvalidSender(address(0));
                revert("ERC20InvalidSender");
            }
            _update(account, address(0), value);
        }

        /**
             * @dev Transfers a `value` amount of tokens from `from` to `to`, or alternatively mints (or burns) if `from`
             * (or `to`) is the zero address. All customizations to transfers, mints, and burns should be done by overriding
             * this function.
             *
             * Emits a {Transfer} event.
             */
            function _update(address from, address to, uint256 value) internal virtual {
                if (from == address(0)) {
                    // Overflow check required: The rest of the code assumes that totalSupply never overflows
                    // _totalSupply += value;
                } else {
                    uint256 fromBalance = balanceOf[from];
                    if (fromBalance < value) {
                        // revert ERC20InsufficientBalance(from, fromBalance, value);
                        revert("ERC20InsufficientBalance");
                    }
                    unchecked {
                        // Overflow not possible: value <= fromBalance <= totalSupply.
                        balanceOf[from] = fromBalance - value;
                    }
                }

                if (to == address(0)) {
                    unchecked {
                        // Overflow not possible: value <= totalSupply or value <= fromBalance <= totalSupply.
                        // _totalSupply -= value;
                    }
                } else {
                    unchecked {
                        // Overflow not possible: balance + value is at most totalSupply, which we know fits into a uint256.
                        balanceOf[to] += value;
                    }
                }

                emit Transfer(from, to, value);
            }
}
