set -eu
# set -x

EOA_ADDRESS="0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
EOA_PRIVATE_KEY="0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
L2_RPC="http://127.0.0.1:8123"

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

sed_inplace "s/zkevm.native-issue-address: \"\"/zkevm.native-issue-address: \"$NATIVE_ISSUE_ADDRESS\"/g" config/test.erigon.seq.config.yaml
sed_inplace "s/zkevm.native-issue-address: \"\"/zkevm.native-issue-address: \"$NATIVE_ISSUE_ADDRESS\"/g" config/test.erigon.rpc.config.yaml

CURRENT_BLOCK=$(cast block latest --rpc-url $L2_RPC | grep number | awk '{print $2}')
echo "Current block number: $CURRENT_BLOCK"
FORK_V1_BLOCK_NUMBER=$(($CURRENT_BLOCK + 20))
sed_inplace "s/zkevm.fork-v1-block-number: [0-9]*/zkevm.fork-v1-block-number: $FORK_V1_BLOCK_NUMBER/g" config/test.erigon.seq.config.yaml
sed_inplace "s/zkevm.fork-v1-block-number: [0-9]*/zkevm.fork-v1-block-number: $FORK_V1_BLOCK_NUMBER/g" config/test.erigon.rpc.config.yaml

echo "Contract address: $NATIVE_ISSUE_ADDRESS, + fork v1 block number: $FORK_V1_BLOCK_NUMBER"


docker-compose stop xlayer-seq
docker-compose stop xlayer-rpc
sleep 10
docker-compose up -d xlayer-seq
docker-compose up -d xlayer-rpc
sleep 10

source .env

while true; do
    block=$(cast block latest --rpc-url $L2_RPC | grep number | awk '{print $2}')
    balance=$(cast balance $NATIVE_ISSUE_ADDRESS --rpc-url $L2_RPC)
    echo "Fork v1 block number: $FORK_V1_BLOCK_NUMBER, current block number: $block, balance: $balance"
    if [ "$block" -gt "$FORK_V1_BLOCK_NUMBER" ]; then
        echo "Fork v1 block number: $FORK_V1_BLOCK_NUMBER, current block number: $block, balance: $balance"
        break
    fi
    sleep 1
done

sleep 10
block=$(cast block latest --rpc-url $L2_RPC | grep number | awk '{print $2}')
balance=$(cast balance $NATIVE_ISSUE_ADDRESS --rpc-url $L2_RPC)
echo "Fork v1 block number: $FORK_V1_BLOCK_NUMBER, current block number: $block, balance: $balance"








