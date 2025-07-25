package types

import (
	"encoding/json"
	"fmt"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type BlockMessage struct {
	Header           *ethTypes.Header
	PrevBlockTxCount int64
	PrevBlockHash    libcommon.Hash
}

func ToKafkaBlockMessage(header *ethTypes.Header, prevBlockTxCount int64, prevBlockHash libcommon.Hash) (blockMsg BlockMessage, err error) {
	blockMsg = BlockMessage{
		Header:           header,
		PrevBlockTxCount: prevBlockTxCount,
		PrevBlockHash:    prevBlockHash,
	}

	return blockMsg, nil
}

func (msg BlockMessage) GetBlockInfo() (*ethTypes.Header, int64, libcommon.Hash, error) {
	if msg.Header == nil {
		return nil, 0, libcommon.Hash{}, fmt.Errorf("header is nil")
	}
	if msg.Header.Number.Uint64() == 0 {
		return nil, 0, libcommon.Hash{}, fmt.Errorf("block number is 0")
	}

	return msg.Header, msg.PrevBlockTxCount, msg.PrevBlockHash, nil
}

func (msg BlockMessage) MarshalJSON() ([]byte, error) {
	type BlockMessage struct {
		Header           *ethTypes.Header `json:"header"`
		PrevBlockTxCount int64            `json:"prevBlockTxCount"`
		PrevBlockHash    libcommon.Hash   `json:"prevBlockHash"`
	}

	var enc BlockMessage
	enc.Header = msg.Header
	enc.PrevBlockTxCount = msg.PrevBlockTxCount
	enc.PrevBlockHash = msg.PrevBlockHash

	return json.Marshal(&enc)
}
