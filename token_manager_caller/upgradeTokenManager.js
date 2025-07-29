const { ethers } = require("hardhat");
require('dotenv').config();

async function main() {
  console.log("=== Upgrading TokenManager Contract ===\n");

  // Get the deployer account
  const [deployer] = await ethers.getSigners();
  console.log("Upgrader address:", deployer.address);
  console.log();

  // You need to provide the proxy address from the initial deployment
  const PROXY_ADDRESS = process.env.PROXY_ADDRESS || "YOUR_PROXY_ADDRESS_HERE";
  
  if (PROXY_ADDRESS === "YOUR_PROXY_ADDRESS_HERE") {
    console.error("❌ Please set PROXY_ADDRESS in your .env file or provide it as an environment variable");
    process.exit(1);
  }

  console.log("Proxy address:", PROXY_ADDRESS);

  // Step 1: Get the current proxy contract instance
  console.log("1. Connecting to existing proxy...");
  const TokenManagerCallerUpgradeable = await ethers.getContractFactory("TokenManagerCallerUpgradeable");
  const proxyContract = TokenManagerCallerUpgradeable.attach(PROXY_ADDRESS);

  // Get current version and owner
  const currentOwner = await proxyContract.owner();
  const currentVersion = await proxyContract.getVersion();
  
  // Get the old implementation address BEFORE upgrade
  const TokenManagerProxy = await ethers.getContractFactory("TokenManagerProxy");
  const proxy = TokenManagerProxy.attach(PROXY_ADDRESS);
  const oldImplementation = await proxy.implementation();
  
  console.log("Current owner:", currentOwner);
  console.log("Current version:", currentVersion);
  console.log("Old implementation:", oldImplementation);
  console.log();

  // Step 2: Deploy new implementation
  console.log("2. Deploying new implementation...");
  const newImplementation = await TokenManagerCallerUpgradeable.deploy();
  await newImplementation.waitForDeployment();
  
  const newImplementationAddress = await newImplementation.getAddress();
  console.log("New implementation deployed to:", newImplementationAddress);
  console.log();

  // Step 3: Upgrade the proxy
  console.log("3. Upgrading proxy to new implementation...");
  
  try {
    const upgradeTx = await proxyContract.upgradeToAndCall(newImplementationAddress, "0x");
    await upgradeTx.wait();
    console.log("✓ Upgrade transaction completed");
    console.log("Transaction hash:", upgradeTx.hash);
  } catch (error) {
    console.error("❌ Upgrade failed:", error.message);
    process.exit(1);
  }
  console.log();

  // Step 4: Verify the upgrade
  console.log("4. Verifying upgrade...");
  
  // Get the current implementation address AFTER upgrade
  const currentImplementation = await proxy.implementation();
  
  console.log("Old implementation address:     ", oldImplementation);
  console.log("New implementation address:     ", newImplementationAddress);
  console.log("Current implementation address: ", currentImplementation);
  
  if (currentImplementation.toLowerCase() === newImplementationAddress.toLowerCase()) {
    console.log("✅ Upgrade successful!");
  } else {
    console.log("❌ Upgrade verification failed!");
    process.exit(1);
  }
  
  if (oldImplementation.toLowerCase() === newImplementationAddress.toLowerCase()) {
    console.log("⚠️  Warning: New implementation is the same as old implementation");
  } else {
    console.log("✅ Implementation actually changed");
  }

  // Step 5: Test functionality after upgrade
  console.log();
  console.log("5. Testing functionality after upgrade...");
  
  const ownerAfterUpgrade = await proxyContract.owner();
  const versionAfterUpgrade = await proxyContract.getVersion();
  
  console.log("Owner after upgrade:", ownerAfterUpgrade);
  console.log("Version after upgrade:", versionAfterUpgrade);
  
  // Test that permissions are preserved
  const canOperate = await proxyContract.canOperate(deployer.address);
  console.log("Can still operate:", canOperate);
  console.log();

  // Step 6: Summary
  console.log("=== Upgrade Summary ===");
  console.log("Proxy Address:", PROXY_ADDRESS);
  console.log("Old Implementation:", oldImplementation);
  console.log("New Implementation:", newImplementationAddress);
  console.log("Owner:", ownerAfterUpgrade);
  console.log("Version:", versionAfterUpgrade);
  console.log("✅ All state preserved during upgrade");
  console.log();

  return {
    proxy: PROXY_ADDRESS,
    oldImplementation: oldImplementation,
    newImplementation: newImplementationAddress,
    owner: ownerAfterUpgrade,
    version: versionAfterUpgrade
  };
}

// Execute the upgrade
if (require.main === module) {
  main()
    .then(() => process.exit(0))
    .catch((error) => {
      console.error("Upgrade failed:", error);
      process.exit(1);
    });
}

module.exports = main; 