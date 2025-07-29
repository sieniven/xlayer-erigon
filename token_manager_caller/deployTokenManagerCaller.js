const { ethers } = require("hardhat");
require('dotenv').config();

async function main() {
  console.log("Deploying TokenManagerCaller contract...");

  // Get the ContractFactory
  const TokenManagerCaller = await ethers.getContractFactory("TokenManagerCaller");

  // Deploy the contract
  const tokenManagerCaller = await TokenManagerCaller.deploy();

  // Wait for deployment to be mined
  await tokenManagerCaller.waitForDeployment();

  const contractAddress = await tokenManagerCaller.getAddress();
  console.log("TokenManagerCaller deployed to:", contractAddress);

  // Get deployment transaction details
  const deploymentTx = tokenManagerCaller.deploymentTransaction();
  console.log("Deployment transaction hash:", deploymentTx.hash);

  // Get the deployer address (this will be the owner)
  const [deployer] = await ethers.getSigners();
  console.log("Contract owner (deployer):", deployer.address);

  // Verify the deployment
  console.log("Verifying deployment...");
  const code = await ethers.provider.getCode(contractAddress);
  if (code === "0x") {
    console.error("Contract deployment failed!");
    process.exit(1);
  } else {
    console.log("Contract deployed successfully!");
  }

  // Verify owner is set correctly
  const owner = await tokenManagerCaller.owner();
  console.log("Contract owner set to:", owner);

  return {
    contract: tokenManagerCaller,
    address: contractAddress,
    owner: owner,
    transactionHash: deploymentTx.hash
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