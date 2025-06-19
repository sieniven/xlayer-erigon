package subscription

import (
	"fmt"
	"math/big"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	proto_realtime "github.com/ledgerwatch/erigon/zk/realtime/subscription/proto"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

func FromProtoTxMessage(reply *proto_realtime.RealtimeTransactionReply) (types.Transaction, *types.Receipt, []*zktypes.InnerTx, error) {
	if reply == nil {
		return nil, nil, nil, fmt.Errorf("protobuf message is nil")
	}

	tx, err := types.DecodeTransaction(reply.RlpTransaction)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode rlp-encoded transaction failed: %v", err)
	}

	var (
		receipt  = &types.Receipt{}
		innerTxs = make([]*zktypes.InnerTx, len(reply.InnerTxs))
	)

	// Convert Receipt
	if reply.Receipt != nil {
		var bloom types.Bloom
		if len(reply.Receipt.LogsBloom) == 256 {
			copy(bloom[:], reply.Receipt.LogsBloom)
		} else if len(reply.Receipt.LogsBloom) != 0 {
			return nil, nil, nil, fmt.Errorf("invalid logsBloom length: %d", len(reply.Receipt.LogsBloom))
		}

		var blockNumber big.Int
		if reply.Receipt.BlockNumber != "" {
			if _, ok := blockNumber.SetString(reply.Receipt.BlockNumber, 10); !ok {
				return nil, nil, nil, fmt.Errorf("invalid blockNumber: %s", reply.Receipt.BlockNumber)
			}
		}

		var txHash common.Hash
		if len(reply.Receipt.TransactionHash) == 32 {
			copy(txHash[:], reply.Receipt.TransactionHash)
		} else if len(reply.Receipt.TransactionHash) != 0 {
			return nil, nil, nil, fmt.Errorf("invalid transactionHash length: %d", len(reply.Receipt.TransactionHash))
		}

		var contractAddress common.Address
		if len(reply.Receipt.ContractAddress) == 20 {
			copy(contractAddress[:], reply.Receipt.ContractAddress)
		} else if len(reply.Receipt.ContractAddress) != 0 {
			return nil, nil, nil, fmt.Errorf("invalid contractAddress length: %d", len(reply.Receipt.ContractAddress))
		}

		var blockHash common.Hash
		if len(reply.Receipt.BlockHash) == 32 {
			copy(blockHash[:], reply.Receipt.BlockHash)
		} else if len(reply.Receipt.BlockHash) != 0 {
			return nil, nil, nil, fmt.Errorf("invalid blockHash length: %d", len(reply.Receipt.BlockHash))
		}

		logs := make([]*types.Log, len(reply.Receipt.Logs))
		for i, protoLog := range reply.Receipt.Logs {
			var address common.Address
			if len(protoLog.Address) == 20 {
				copy(address[:], protoLog.Address)
			} else if len(protoLog.Address) != 0 {
				return nil, nil, nil, fmt.Errorf("invalid log address length: %d", len(protoLog.Address))
			}

			var logTxHash common.Hash
			if len(protoLog.TransactionHash) == 32 {
				copy(logTxHash[:], protoLog.TransactionHash)
			} else if len(protoLog.TransactionHash) != 0 {
				return nil, nil, nil, fmt.Errorf("invalid log transactionHash length: %d", len(protoLog.TransactionHash))
			}

			var logBlockHash common.Hash
			if len(protoLog.BlockHash) == 32 {
				copy(logBlockHash[:], protoLog.BlockHash)
			} else if len(protoLog.BlockHash) != 0 {
				return nil, nil, nil, fmt.Errorf("invalid log blockHash length: %d", len(protoLog.BlockHash))
			}

			topics := make([]common.Hash, len(protoLog.Topics))
			for j, topic := range protoLog.Topics {
				if len(topic) == 32 {
					var hash common.Hash
					copy(hash[:], topic)
					topics[j] = hash
				} else if len(topic) != 0 {
					return nil, nil, nil, fmt.Errorf("invalid topic length: %d", len(topic))
				}
			}

			logs[i] = &types.Log{
				Address:     address,
				Topics:      topics,
				Data:        protoLog.Data,
				BlockNumber: protoLog.BlockNumber,
				TxHash:      logTxHash,
				TxIndex:     uint(protoLog.TransactionIndex),
				BlockHash:   logBlockHash,
				Index:       uint(protoLog.LogIndex),
				Removed:     protoLog.Removed,
			}
		}

		receipt = &types.Receipt{
			Type:              uint8(reply.Receipt.Type),
			PostState:         reply.Receipt.Root,
			Status:            reply.Receipt.Status,
			CumulativeGasUsed: reply.Receipt.CumulativeGasUsed,
			Bloom:             bloom,
			Logs:              logs,
			TxHash:            txHash,
			ContractAddress:   contractAddress,
			GasUsed:           reply.Receipt.GasUsed,
			BlockHash:         blockHash,
			BlockNumber:       &blockNumber,
			TransactionIndex:  uint(reply.Receipt.TransactionIndex),
		}
	}

	// Convert InnerTxs
	for i, protoInnerTx := range reply.InnerTxs {
		var dept big.Int
		if protoInnerTx.Dept != "" {
			if _, ok := dept.SetString(protoInnerTx.Dept, 10); !ok {
				return nil, nil, nil, fmt.Errorf("invalid dept: %s", protoInnerTx.Dept)
			}
		}

		var internalIndex big.Int
		if protoInnerTx.InternalIndex != "" {
			if _, ok := internalIndex.SetString(protoInnerTx.InternalIndex, 10); !ok {
				return nil, nil, nil, fmt.Errorf("invalid internal_index: %s", protoInnerTx.InternalIndex)
			}
		}

		innerTxs[i] = &zktypes.InnerTx{
			Dept:          dept,
			InternalIndex: internalIndex,
			CallType:      protoInnerTx.CallType,
			Name:          protoInnerTx.Name,
			TraceAddress:  protoInnerTx.TraceAddress,
			CodeAddress:   protoInnerTx.CodeAddress,
			From:          protoInnerTx.From,
			To:            protoInnerTx.To,
			Input:         protoInnerTx.Input,
			Output:        protoInnerTx.Output,
			IsError:       protoInnerTx.IsError,
			Gas:           protoInnerTx.Gas,
			GasUsed:       protoInnerTx.GasUsed,
			Value:         protoInnerTx.Value,
			ValueWei:      protoInnerTx.ValueWei,
			CallValueWei:  protoInnerTx.CallValueWei,
			Error:         protoInnerTx.Error,
		}
	}

	return tx, receipt, innerTxs, nil
}
