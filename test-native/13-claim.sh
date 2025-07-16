set -eu
# set -x

source .env

EOA_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
EOA_PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
L2_RPC="http://127.0.0.1:8123"

before_balance=$(cast balance $NATIVE_ISSUE_ADDRESS --rpc-url $L2_RPC)
eoa_balance=$(cast balance $EOA_ADDRESS --rpc-url $L2_RPC)

echo "Calling Claim function to get 100 OKB..."
cast send $NATIVE_ISSUE_ADDRESS "Claim(uint256)" 100000000000000000000 --private-key $EOA_PRIVATE_KEY --rpc-url $L2_RPC --legacy
after_balance=$(cast balance $NATIVE_ISSUE_ADDRESS --rpc-url $L2_RPC)
after_eoa_balance=$(cast balance $EOA_ADDRESS --rpc-url $L2_RPC)

echo "Contract balance: $before_balance, $after_balance" 
echo "EOA balance: $eoa_balance, $after_eoa_balance" 
