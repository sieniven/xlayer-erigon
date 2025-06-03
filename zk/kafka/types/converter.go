package types

import (
	"encoding/hex"
	"fmt"

	types1 "github.com/ledgerwatch/erigon/core/types"
)

func fromCommonTxMessage(tx types1.Transaction, blockNumber uint64) (TransactionMessage, error) {
	msg := TransactionMessage{
		BlockNumber: blockNumber,
		Type:        tx.Type(),
		Hash:        tx.Hash().String(),
		ChainID:     tx.GetChainID().Uint64(),
		Nonce:       tx.GetNonce(),
		Gas:         tx.GetGas(),
		To:          tx.GetTo().String(),
		Value:       tx.GetValue().String(),
		Data:        hex.EncodeToString(tx.GetData()),
	}

	txSender, ok := tx.GetSender()
	if !ok {
		return TransactionMessage{}, fmt.Errorf("failed to recover sender from transaction")
	}
	msg.From = txSender.String()

	v, r, s := tx.RawSignatureValues()
	msg.V = v.String()
	msg.R = r.String()
	msg.S = s.String()

	return msg, nil
}

func fromLegacyTxMessage(tx types1.Transaction, blockNumber uint64) (TransactionMessage, error) {
	msg, err := fromCommonTxMessage(tx, blockNumber)
	if err != nil {
		return TransactionMessage{}, err
	}

	msg.GasPrice = tx.GetPrice().String()

	return msg, nil
}

func fromAccessListTxMessage(tx types1.Transaction, blockNumber uint64) (TransactionMessage, error) {
	msg, err := fromLegacyTxMessage(tx, blockNumber)
	if err != nil {
		return TransactionMessage{}, err
	}

	accessList := tx.GetAccessList()
	msg.AccessList = make([]AccessTupleMessage, 0, len(accessList))
	for _, tuple := range accessList {
		msg.AccessList = append(msg.AccessList, fromAccessTuple(tuple))
	}

	return msg, nil
}

func fromDynamicFeeTxMessage(tx types1.Transaction, blockNumber uint64) (TransactionMessage, error) {
	msg, err := fromCommonTxMessage(tx, blockNumber)
	if err != nil {
		return TransactionMessage{}, err
	}

	accessList := tx.GetAccessList()
	msg.AccessList = make([]AccessTupleMessage, 0, len(accessList))
	for _, tuple := range accessList {
		msg.AccessList = append(msg.AccessList, fromAccessTuple(tuple))
	}

	msg.Tip = tx.GetTip().String()
	msg.FeeCap = tx.GetFeeCap().String()

	return msg, nil
}

func fromBlobTxMessage(tx types1.Transaction, blockNumber uint64) (TransactionMessage, error) {
	// Check if it's a BlobTx or BlobTxWrapper
	msg, err := fromDynamicFeeTxMessage(tx, blockNumber)
	if err != nil {
		return TransactionMessage{}, err
	}

	switch t := tx.(type) {
	case *types1.BlobTx:
		msg.BlobTxAreWrappedWithBlobs = false
		msg.MaxFeePerBlobGas = t.MaxFeePerBlobGas.String()
		msg.BlobVersionedHashes = make([]string, 0, len(t.BlobVersionedHashes))
		for _, hash := range t.BlobVersionedHashes {
			msg.BlobVersionedHashes = append(msg.BlobVersionedHashes, hash.String())
		}
	case *types1.BlobTxWrapper:
		msg.BlobTxAreWrappedWithBlobs = true
		msg.MaxFeePerBlobGas = t.Tx.MaxFeePerBlobGas.String()
		msg.BlobVersionedHashes = make([]string, 0, len(t.Tx.BlobVersionedHashes))
		for _, hash := range t.Tx.BlobVersionedHashes {
			msg.BlobVersionedHashes = append(msg.BlobVersionedHashes, hash.String())
		}
	default:
		return TransactionMessage{}, fmt.Errorf("unsupported transaction type: %d", tx.Type())
	}

	return msg, nil
}
