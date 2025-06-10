package types

import (
	"encoding/json"
	"fmt"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core/types"
	types1 "github.com/ledgerwatch/erigon/core/types"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

type TxInfo struct {
	Transaction types.Transaction
	Receipt     *types.Receipt
	InnerTxs    []*zktypes.InnerTx
	Changeset   *zktypes.Changeset
}

// TransactionMessage represents the structure of the transaction message to be sent to Kafka
type TransactionMessage struct {
	// Sequenced block number
	BlockNumber uint64 `json:"blockNumber"`

	// Common tx fields
	Type    uint8              `json:"type"`
	Hash    libcommon.Hash     `json:"hash"`
	From    libcommon.Address  `json:"from"`
	ChainID *uint256.Int       `json:"chainId"`
	Nonce   uint64             `json:"nonce"`
	Gas     uint64             `json:"gas"`
	To      *libcommon.Address `json:"to"`
	Value   *uint256.Int       `json:"value"`
	Data    []byte             `json:"data"`
	V       uint256.Int        `json:"v"`
	R       uint256.Int        `json:"r"`
	S       uint256.Int        `json:"s"`

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
	Receipt *types1.Receipt `json:"receipt"`

	// Inner transactions
	InnerTxs []*zktypes.InnerTx `json:"innerTxs"`

	// Changeset
	Changeset *zktypes.Changeset `json:"changeset"`
}

func ToKafkaTransactionMessage(tx types1.Transaction, receipt *types1.Receipt, innerTxs []*zktypes.InnerTx, changeset *zktypes.Changeset, blockNumber uint64) (txMsg TransactionMessage, err error) {
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
	txMsg.InnerTxs = innerTxs
	txMsg.Changeset = changeset

	return txMsg, nil
}

func (msg TransactionMessage) GetTransaction() (types1.Transaction, uint64, error) {
	blockNumber := msg.BlockNumber

	// Get tx
	switch msg.Type {
	case types1.LegacyTxType:
		tx, err := msg.toLegacyTx()
		if err != nil {
			return nil, blockNumber, err
		}

		return &tx, blockNumber, nil
	case types1.AccessListTxType:
		tx, err := msg.toAccessListTx()
		if err != nil {
			return nil, blockNumber, err
		}

		return &tx, blockNumber, nil
	case types1.DynamicFeeTxType:
		tx, err := msg.toDynamicFeeTx()
		if err != nil {
			return nil, blockNumber, err
		}

		return &tx, blockNumber, nil
	case types1.BlobTxType:
		tx, err := msg.toBlobTx()
		if err != nil {
			return nil, blockNumber, err
		}

		return &tx, blockNumber, nil
	default:
		return nil, blockNumber, fmt.Errorf("unsupported transaction type: %d", msg.Type)
	}
}

func (msg TransactionMessage) GetReceipt() (*types1.Receipt, error) {
	if msg.Receipt == nil {
		return nil, fmt.Errorf("receipt is nil")
	}

	return msg.Receipt, nil
}

func (msg TransactionMessage) GetInnerTxs() ([]*zktypes.InnerTx, error) {
	if msg.InnerTxs == nil {
		return nil, fmt.Errorf("innerTxs is nil")
	}

	return msg.InnerTxs, nil
}

func (msg TransactionMessage) GetChangeset() (*zktypes.Changeset, error) {
	if msg.Changeset == nil {
		return nil, fmt.Errorf("changeset is nil")
	}

	return msg.Changeset, nil
}

func (msg TransactionMessage) MarshalJSON() ([]byte, error) {
	type TransactionMessage struct {
		BlockNumber uint64             `json:"blockNumber"`
		Type        uint8              `json:"type"`
		Hash        libcommon.Hash     `json:"hash"`
		From        libcommon.Address  `json:"from"`
		ChainID     *uint256.Int       `json:"chainId"`
		Nonce       uint64             `json:"nonce"`
		Gas         uint64             `json:"gas"`
		To          *libcommon.Address `json:"to"`
		Value       *uint256.Int       `json:"value"`
		Data        []byte             `json:"data"`
		V           uint256.Int        `json:"v"`
		R           uint256.Int        `json:"r"`
		S           uint256.Int        `json:"s"`
		GasPrice    string             `json:"gasPrice"`
		Receipt     *types1.Receipt    `json:"receipt"`
		InnerTxs    []*zktypes.InnerTx `json:"innerTxs"`
		Changeset   *zktypes.Changeset `json:"changeset"`
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
	enc.InnerTxs = msg.InnerTxs
	enc.Changeset = msg.Changeset

	if msg.Receipt != nil {
		// Handle nil logs
		receipt := *msg.Receipt
		if receipt.Logs == nil {
			receipt.Logs = []*types1.Log{}
		}

		// Handle nil topics
		for _, log := range receipt.Logs {
			if log != nil {
				if log.Topics == nil {
					log.Topics = []libcommon.Hash{}
				}
			}
		}
		enc.Receipt = &receipt
	}

	return json.Marshal(&enc)
}
