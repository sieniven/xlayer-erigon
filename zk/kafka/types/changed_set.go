package types

import (
	"github.com/ledgerwatch/erigon-lib/common"
	historyv22 "github.com/ledgerwatch/erigon-lib/kv/temporal/historyv2"
)

type ChangedSet struct {
	TxHash            common.Hash           `json:"txHash"`
	AccountChangedSet *historyv22.ChangeSet `json:"accountChangedSet"`
	StorageChangedSet *historyv22.ChangeSet `json:"storageChangedSet"`
}
