#!/bin/bash
set -e
set -x

# cast calldata "updateRollup(address,uint32,bytes)" \
# >   0xeb173087729c88a47568AF87b17C653039377BA6 \
# >   14 \
# >   0x
# 0xc4c928c2000000000000000000000000eb173087729c88a47568af87b17c653039377ba6000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000

# curl --location 'http://127.0.0.1:8545' \
# --header 'Content-Type: application/json' \
# --data '{
# 	"jsonrpc":"2.0",
# 	"method":"eth_call",
# 	"params":[{
# 		"from": "0x8f8E2d6cF621f30e9a11309D6A56A876281Fd534",
# 		"to": "0x2d42E2899662EFf08b13eeb65b154b904C7a1c8a",
# 		"gas": "0x493e0",
# 		"gasPrice": "0x29e8d60800",
# 		"value": "0x0",
# 		"data":"0xc4c928c2000000000000000000000000eb173087729c88a47568af87b17c653039377ba6000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000600000000000000000000000000000000000000000000000000000000000000000"
# 	}, "latest"],
# 	"id":1
# }'

cast sig "RollupTypeDoesNotExist()"
cast sig "RollupMustExist()"
cast sig "UpdateToSameRollupTypeID()"
cast sig "RollupTypeObsolete()"
cast sig "UpdateNotCompatible()"
cast sig "function verifyPessimisticTrustedAggregator(uint32 rollupID,uint32 l1InfoTreeLeafCount,bytes32 newLocalExitRoot,bytes32 newPessimisticRoot,bytes calldata proof,bytes calldata aggchainData)"