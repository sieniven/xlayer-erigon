const { ethers } = require("hardhat");
require('dotenv').config();

async function predictContractAddress() {
  console.log("=== Contract Address Prediction Tool ===\n");
  
  // Get deployer information from private key in config
  const deployer = new ethers.Wallet(process.env.PRIVATE_KEY, ethers.provider);
  const currentNonce = await deployer.getNonce();
  
  console.log("Deployer address:", deployer.address);
  console.log("Current nonce:", currentNonce);
  console.log();
  
  // Predict normal deployment addresses
  console.log("=== Normal Deployment (CREATE) ===");
  const normalAddress = ethers.getCreateAddress({
    from: deployer.address,
    nonce: currentNonce
  });
  console.log("Predicted contract address:", normalAddress);
  console.log("Deploy command: npx hardhat run deployTokenManagerCaller.js --network xlayer");
  console.log();
  
  // Predict upgradeable deployment addresses
  console.log("=== Upgradeable Deployment ===");
  const implementationAddress = ethers.getCreateAddress({
    from: deployer.address,
    nonce: currentNonce
  });
  
  const proxyAddress = ethers.getCreateAddress({
    from: deployer.address,
    nonce: currentNonce + 1
  });
  
  console.log("Predicted implementation address:", implementationAddress);
  console.log("Predicted proxy address:", proxyAddress);
  console.log("Deploy command: npx hardhat run deployUpgradeableTokenManager.js --network xlayer");
  console.log();
  console.log("⚠️  NOTE: Use the PROXY address for all interactions, not the implementation address!");
  console.log();
  
  // Predict consecutive deployment addresses
  console.log("=== Consecutive Deployment Address Prediction ===");
  for (let i = 0; i < 5; i++) {
    const futureAddress = ethers.getCreateAddress({
      from: deployer.address,
      nonce: currentNonce + i
    });
    console.log(`Deploy ${i + 1} (nonce ${currentNonce + i}): ${futureAddress}`);
  }
  console.log();
  
  // Return prediction result
  return {
    deployerAddress: deployer.address,
    currentNonce: currentNonce,
    normalDeployment: normalAddress,
    upgradeableImplementation: implementationAddress,
    upgradeableProxy: proxyAddress
  };
}

// Main function
if (require.main === module) {
  predictContractAddress()
    .then(() => process.exit(0))
    .catch((error) => {
      console.error("Prediction failed:", error);
      process.exit(1);
    });
}

module.exports = { predictContractAddress }; 