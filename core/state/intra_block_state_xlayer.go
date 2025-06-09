package state

import (
	"fmt"
	"sort"

	"github.com/ledgerwatch/erigon/core/types"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
)

func (sdb *IntraBlockState) GenerateChangesetSinceSnapshotAndSendTxInfo(revid int, txMsgChan chan *zktypes.TxInfo, tx types.Transaction, receipt *types.Receipt, innerTxs []*zktypes.InnerTx) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(sdb.validRevisions), func(i int) bool {
		return sdb.validRevisions[i].id >= revid
	})
	if idx == len(sdb.validRevisions) || sdb.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := sdb.validRevisions[idx].journalIndex
	entries := &sdb.journal.entries

	go func() {
		changeset := zktypes.NewChangeset()
		for _, entry := range (*entries)[snapshot:] {
			entry.collectChangeset(changeset)
		}

		txMsgChan <- &zktypes.TxInfo{
			BlockNumber: receipt.BlockNumber.Uint64(),
			Tx:          tx,
			Receipt:     receipt,
			InnerTxs:    innerTxs,
			Changeset:   changeset,
		}
	}()
}
