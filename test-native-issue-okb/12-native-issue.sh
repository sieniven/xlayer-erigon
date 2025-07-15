set -eu
# set -x

EOA_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
EOA_PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
L2_RPC="http://127.0.0.1:8124"

sed_inplace() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

cd contracts
result=$(forge create NativeIssue.sol:NativeIssue --private-key $EOA_PRIVATE_KEY --rpc-url $L2_RPC --legacy --broadcast)
HASH=$(echo "$result" | grep "Transaction hash:" | awk '{print $3}')
NATIVE_ISSUE_ADDRESS=$(echo "$result" | grep "Deployed to:" | awk '{print $3}')

if [ ! -f .env ]; then
    touch .env
fi


cd ..
if grep -q "NATIVE_ISSUE_ADDRESS=" .env; then
    sed_inplace "s/NATIVE_ISSUE_ADDRESS=.*/NATIVE_ISSUE_ADDRESS=$NATIVE_ISSUE_ADDRESS/" .env
else
    echo "NATIVE_ISSUE_ADDRESS=$NATIVE_ISSUE_ADDRESS" >> .env
fi

echo "Contract address: $NATIVE_ISSUE_ADDRESS"
