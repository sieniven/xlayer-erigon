// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract NativeIssue {
    event Claimed(address indexed user, uint256 amount);
    
    function Claim(uint256 amount) external {
        require(amount > 0, "Amount must be greater than 0");
        require(address(this).balance >= amount, "Insufficient contract balance");
        
        payable(msg.sender).transfer(amount);
        
        emit Claimed(msg.sender, amount);
    }
}