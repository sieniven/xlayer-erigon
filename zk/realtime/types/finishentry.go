package types

import libcommon "github.com/ledgerwatch/erigon-lib/common"

type FinishedEntry struct {
	Height    uint64
	BlockHash libcommon.Hash
}
