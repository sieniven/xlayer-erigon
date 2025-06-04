package types

import (
	"encoding/json"
	"fmt"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	types1 "github.com/ledgerwatch/erigon/core/types"
)

// TransactionMessage represents the structure of the transaction message to be sent to Kafka
type TransactionMessage struct {
	// Sequenced block number
	BlockNumber uint64 `json:"blockNumber"`

	// Common tx fields
	Type    uint8  `json:"type"`
	Hash    string `json:"hash"`
	From    string `json:"from"`
	ChainID uint64 `json:"chainId"`
	Nonce   uint64 `json:"nonce"`
	Gas     uint64 `json:"gas"`
	To      string `json:"to"`
	Value   string `json:"value"`
	Data    string `json:"data"`
	V       string `json:"v"`
	R       string `json:"r"`
	S       string `json:"s"`

	// For legacy txs
	GasPrice string `json:"gasPrice"`
	// For EIP-1559 and EIP-2930 txs
	AccessList []AccessTupleMessage `json:"accessList"`
	Tip        string               `json:"tip"`
	FeeCap     string               `json:"feeCap"`
	// For blob txs
	MaxFeePerBlobGas    string   `json:"maxFeePerBlobGas"`
	BlobVersionedHashes []string `json:"blobVersionedHashes"`

	// Receipt data
	Receipt *types1.Receipt `json:"rlpReceipt"`
}

func ToKafkaTransactionMessage(tx types1.Transaction, receipt *types1.Receipt, blockNumber uint64) (txMsg TransactionMessage, err error) {
	// Parse tx
	switch tx.Type() {
	case types1.LegacyTxType:
		if _, ok := tx.(*types1.LegacyTx); !ok {
			return TransactionMessage{}, fmt.Errorf("incorrect type, failed to encode legacy tx")
		}

		txMsg, err = fromLegacyTxMessage(tx, blockNumber)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse legacy tx error: %w", err)
		}
	case types1.AccessListTxType:
		if _, ok := tx.(*types1.AccessListTx); !ok {
			return TransactionMessage{}, fmt.Errorf("incorrect type, failed to encode access list tx")
		}

		txMsg, err = fromAccessListTxMessage(tx, blockNumber)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse accesslist tx error: %w", err)
		}
	case types1.DynamicFeeTxType:
		if _, ok := tx.(*types1.DynamicFeeTransaction); !ok {
			return TransactionMessage{}, fmt.Errorf("incorrect type, failed to encode dynamic fee tx")
		}

		txMsg, err = fromDynamicFeeTxMessage(tx, blockNumber)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse dynamic fee tx error: %w", err)
		}
	case types1.BlobTxType:
		switch tx.(type) {
		case *types1.BlobTx:
			// continue
		case *types1.BlobTxWrapper:
			// continue
		default:
			return TransactionMessage{}, fmt.Errorf("incorrect type, failed to encode blob tx")
		}

		txMsg, err = fromBlobTxMessage(tx, blockNumber)
		if err != nil {
			return TransactionMessage{}, fmt.Errorf("parse blob tx error: %w", err)
		}
	default:
		return TransactionMessage{}, fmt.Errorf("unsupported transaction type: %d", tx.Type())
	}

	// Parse receipt
	txMsg.Receipt = receipt

	return txMsg, nil
}

func (msg TransactionMessage) GetTransaction() (types1.Transaction, uint64, *types1.Receipt, error) {
	blockNumber := msg.BlockNumber
	receipt := msg.Receipt

	// Get tx
	switch msg.Type {
	case types1.LegacyTxType:
		tx, err := msg.toLegacyTx()
		if err != nil {
			return nil, blockNumber, receipt, err
		}

		return &tx, blockNumber, receipt, nil
	case types1.AccessListTxType:
		tx, err := msg.toAccessListTx()
		if err != nil {
			return nil, blockNumber, receipt, err
		}

		return &tx, blockNumber, receipt, nil
	case types1.DynamicFeeTxType:
		tx, err := msg.toDynamicFeeTx()
		if err != nil {
			return nil, blockNumber, receipt, err
		}

		return &tx, blockNumber, receipt, nil
	case types1.BlobTxType:
		tx, err := msg.toBlobTx()
		if err != nil {
			return nil, blockNumber, receipt, err
		}

		return &tx, blockNumber, receipt, nil
	default:
		return nil, blockNumber, receipt, fmt.Errorf("unsupported transaction type: %d", msg.Type)
	}
}

func (msg TransactionMessage) MarshalJSON() ([]byte, error) {
	type TransactionMessage struct {
		BlockNumber uint64          `json:"blockNumber"`
		Type        uint8           `json:"type"`
		Hash        string          `json:"hash"`
		From        string          `json:"from"`
		ChainID     uint64          `json:"chainId"`
		Nonce       uint64          `json:"nonce"`
		Gas         uint64          `json:"gas"`
		To          string          `json:"to"`
		Value       string          `json:"value"`
		Data        string          `json:"data"`
		R           string          `json:"r"`
		S           string          `json:"s"`
		V           string          `json:"v"`
		GasPrice    string          `json:"gasPrice"`
		Receipt     *types1.Receipt `json:"receipt"`
	}

	var enc TransactionMessage
	enc.BlockNumber = msg.BlockNumber
	enc.Type = msg.Type
	enc.Hash = msg.Hash
	enc.From = msg.From
	enc.ChainID = msg.ChainID
	enc.Nonce = msg.Nonce
	enc.Gas = msg.Gas
	enc.To = msg.To
	enc.Value = msg.Value
	enc.Data = msg.Data
	enc.R = msg.R
	enc.S = msg.S
	enc.V = msg.V
	enc.GasPrice = msg.GasPrice

	if msg.Receipt != nil {
		receipt := *msg.Receipt
		if receipt.Logs != nil {
			logs := make([]*types1.Log, len(receipt.Logs))
			for i, log := range receipt.Logs {
				if log != nil {
					logCopy := *log
					if logCopy.Topics == nil {
						logCopy.Topics = []libcommon.Hash{}
					}
					logs[i] = &logCopy
				}
			}
			receipt.Logs = logs
		}
		enc.Receipt = &receipt
	}

	return json.Marshal(&enc)
}
