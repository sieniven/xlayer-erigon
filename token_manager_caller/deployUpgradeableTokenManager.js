const { ethers } = require("hardhat");
require('dotenv').config();

async function main() {
  console.log("=== Deploying Upgradeable TokenManager ===\n");

  // Get the deployer account
  const [deployer] = await ethers.getSigners();
  console.log("Deployer address:", deployer.address);
  console.log("Deployer balance:", ethers.formatEther(await ethers.provider.getBalance(deployer.address)), "ETH");
  console.log();

  // Step 1: Deploy the implementation contract
  console.log("1. Deploying TokenManagerCallerUpgradeable implementation...");
  const TokenManagerCallerUpgradeable = await ethers.getContractFactory("TokenManagerCallerUpgradeable");
  const implementation = await TokenManagerCallerUpgradeable.deploy();
  await implementation.waitForDeployment();
  
  const implementationAddress = await implementation.getAddress();
  console.log("Implementation deployed to:", implementationAddress);
  console.log();

  // Step 2: Prepare initialization data
  console.log("2. Preparing initialization data...");
  const initializeData = implementation.interface.encodeFunctionData("initialize", [deployer.address]);
  console.log("Initialize data:", initializeData);
  console.log();

  // Step 3: Deploy the proxy contract
  console.log("3. Deploying TokenManagerProxy...");
  const TokenManagerProxy = await ethers.getContractFactory("TokenManagerProxy");
  const proxy = await TokenManagerProxy.deploy(implementationAddress, initializeData);
  await proxy.waitForDeployment();
  
  const proxyAddress = await proxy.getAddress();
  console.log("Proxy deployed to:", proxyAddress);
  console.log();

  // Step 4: Get the proxy contract instance
  console.log("4. Setting up proxy instance...");
  const proxyContract = TokenManagerCallerUpgradeable.attach(proxyAddress);
  
  // Verify initialization
  const owner = await proxyContract.owner();
  const version = await proxyContract.getVersion();
  console.log("Proxy owner:", owner);
  console.log("Contract version:", version);
  console.log("Implementation address from proxy:", await proxy.implementation());
  console.log();

  // Step 5: Test basic functionality
  console.log("5. Testing basic functionality...");
  
  // Grant operate permission to deployer
  const grantOperateTx = await proxyContract.grantOperatePermission(deployer.address);
  await grantOperateTx.wait();
  console.log("✓ Granted operate permission to deployer");
  
  // Grant burn target permission to deployer (so deployer can be burned from)
  const grantBurnTargetTx = await proxyContract.grantBurnTargetPermission("0x0000000000000000000000000000000000000000");
  await grantBurnTargetTx.wait();
  console.log("✓ Granted burn target permission to deployer");
  
  // Check permissions
  const canOperate = await proxyContract.canOperate(deployer.address);
  const canBeBurned = await proxyContract.canBeBurned("0x0000000000000000000000000000000000000000");
  console.log("✓ Can operate:", canOperate);
  console.log("✓ Can be burned:", canBeBurned);
  console.log();

  // Step 6: Summary
  console.log("=== Deployment Summary ===");
  console.log("Implementation Contract:", implementationAddress);
  console.log("Proxy Contract:", proxyAddress);
  console.log("Contract Owner:", owner);
  console.log("Contract Version:", version);
  console.log();
  
  console.log("=== Next Steps ===");
  console.log("1. Use the PROXY address for all interactions:", proxyAddress);
  console.log("2. To upgrade, deploy a new implementation and call upgradeTo()");
  console.log("3. Grant permissions using grantOperatePermission() and grantBurnTargetPermission()");
  console.log();
  
  console.log("=== Usage Commands ===");
  console.log(`# Grant operate permission (allows mint & burn operations):`);
  console.log(`cast send ${proxyAddress} "grantOperatePermission(address)" <USER_ADDRESS> --private-key <PRIVATE_KEY> --rpc-url <RPC_URL>`);
  console.log();
  console.log(`# Grant burn target permission (allows address to be burned from):`);
  console.log(`cast send ${proxyAddress} "grantBurnTargetPermission(address)" <TARGET_ADDRESS> --private-key <PRIVATE_KEY> --rpc-url <RPC_URL>`);
  console.log();
  console.log(`# Mint tokens:`);
  console.log(`cast send ${proxyAddress} "mint(address,uint256)" <TO_ADDRESS> <AMOUNT> --private-key <PRIVATE_KEY> --rpc-url <RPC_URL>`);
  console.log();
  console.log(`# Burn tokens:`);
  console.log(`cast send ${proxyAddress} "burn(address,uint256)" <FROM_ADDRESS> <AMOUNT> --private-key <PRIVATE_KEY> --rpc-url <RPC_URL>`);
  console.log();
  console.log(`# Check implementation:`);
  console.log(`cast call ${proxyAddress} "implementation()(address)" --rpc-url <RPC_URL>`);
  console.log();

  return {
    implementation: implementationAddress,
    proxy: proxyAddress,
    owner: owner,
    version: version
  };
}

// Execute the deployment
if (require.main === module) {
  main()
    .then(() => process.exit(0))
    .catch((error) => {
      console.error("Deployment failed:", error);
      process.exit(1);
    });
}

module.exports = main; 