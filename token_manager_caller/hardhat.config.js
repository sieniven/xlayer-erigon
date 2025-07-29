require("@nomicfoundation/hardhat-toolbox");
require('dotenv').config();

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: "0.8.28",
  networks: {
    xlayer: {
      url: `http://localhost:8123`,
      accounts: [process.env.PRIVATE_KEY]
    }
  }
};
