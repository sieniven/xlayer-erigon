package state

import (
	"fmt"
	"sort"

	"github.com/ledgerwatch/erigon/core/types"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
	"github.com/ledgerwatch/erigon/zkevm/log"
)

func (sdb *IntraBlockState) GenerateChangesetSinceSnapshot(revid int, txMsgChan chan *zktypes.TxInfo, tx types.Transaction, receipt *types.Receipt, innerTxs []*zktypes.InnerTx) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(sdb.validRevisions), func(i int) bool {
		return sdb.validRevisions[i].id >= revid
	})
	if idx == len(sdb.validRevisions) || sdb.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := sdb.validRevisions[idx].journalIndex

	changeset := zktypes.NewChangeset()
	entries := &sdb.journal.entries

	// sdb.journal.changeset(changeset, snapshot)
	go func() {
		fillingChangeset(changeset, entries, snapshot)
		log.Info("Kafka prepare to send transactio", "txhash", tx.Hash(), "innerTxs", innerTxs, "changeset", changeset)

		txMsgChan <- &zktypes.TxInfo{
			BlockNumber: receipt.BlockNumber.Uint64(),
			Tx:          tx,
			Receipt:     receipt,
			InnerTxs:    innerTxs,
			Changeset:   changeset,
		}
	}()
}

func fillingChangeset(changeset *zktypes.Changeset, entries *[]journalEntry, snapshot int) {
	temp := *entries
	for _, entry := range temp[snapshot:] {
		entry.collectChangeset(changeset)
	}
}
