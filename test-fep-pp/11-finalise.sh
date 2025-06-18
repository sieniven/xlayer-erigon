#!/bin/bash
set -e
set -x

DESTINATION_NET="0"
bridge_api_url=http://127.0.0.1:8080
claim_sig="claimAsset(bytes32[32],bytes32[32],uint256,bytes32,bytes32,uint32,address,uint32,address,uint256,bytes)"
curl -s "$bridge_api_url/bridges/$ADDR?limit=100&offset=0" | jq > bridge-deposits.json
cat bridge-deposits.json
jq '[.deposits[] | select(.ready_for_claim == true and .claim_tx_hash == "" and .dest_net == '$DESTINATION_NET')]' bridge-deposits.json > claimable-txs.json
cat claimable-txs.json

tx=$(jq -c '.[0]' claimable-txs.json)

curr_deposit_cnt="$(echo "$tx" | jq -r '.deposit_cnt')"
curr_network_id="$(echo "$tx" | jq -r '.network_id')"
curl -s "$bridge_api_url/merkle-proof?deposit_cnt=$curr_deposit_cnt&net_id=$curr_network_id" | jq '.' > proof.json
cat proof.json

in_merkle_proof="$(jq -r -c '.proof.merkle_proof' proof.json | tr -d '"')"
in_rollup_merkle_proof="$(jq -r -c '.proof.rollup_merkle_proof' proof.json | tr -d '"')"
in_global_index="$(echo "$tx" | jq -r '.global_index')"
in_main_exit_root="$(jq -r '.proof.main_exit_root' proof.json)"
in_rollup_exit_root="$(jq -r '.proof.rollup_exit_root' proof.json)"
in_orig_net="$(echo "$tx" | jq -r '.orig_net')"
in_orig_addr="$(echo "$tx" | jq -r '.orig_addr')"
in_dest_net="$(echo "$tx" | jq -r '.dest_net')"
in_dest_addr="$(echo "$tx" | jq -r '.dest_addr')"
in_amount="$(echo "$tx" | jq -r '.amount')"
in_metadata="$(echo "$tx" | jq -r '.metadata')"

echo "in_merkle_proof: $in_merkle_proof"

cast calldata "$claim_sig" "$in_merkle_proof" "$in_rollup_merkle_proof" "$in_global_index" "$in_main_exit_root" "$in_rollup_exit_root" "$in_orig_net" "$in_orig_addr" "$in_dest_net" "$in_dest_addr" "$in_amount" "$in_metadata"

cast call --rpc-url "$RPC_L1" "$BRIDGE_L2" "$claim_sig" "$in_merkle_proof" "$in_rollup_merkle_proof" "$in_global_index" "$in_main_exit_root" "$in_rollup_exit_root" "$in_orig_net" "$in_orig_addr" "$in_dest_net" "$in_dest_addr" "$in_amount" "$in_metadata"

cast send --rpc-url "$RPC_L1" --private-key "$ADDR_PRIVATE_KEY" "$BRIDGE_L2" "$claim_sig" "$in_merkle_proof" "$in_rollup_merkle_proof" "$in_global_index" "$in_main_exit_root" "$in_rollup_exit_root" "$in_orig_net" "$in_orig_addr" "$in_dest_net" "$in_dest_addr" "$in_amount" "$in_metadata"