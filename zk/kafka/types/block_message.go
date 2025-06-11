package types

import (
	"encoding/json"
	"fmt"

	ethTypes "github.com/ledgerwatch/erigon/core/types"
)

type BlockMessage struct {
	header           *ethTypes.Header
	prevBlockTxCount uint64
}

func ToKafkaBlockMessage(header *ethTypes.Header, prevBlockTxCount uint64) (blockMsg BlockMessage, err error) {
	blockMsg = BlockMessage{
		header:           header,
		prevBlockTxCount: prevBlockTxCount,
	}

	return blockMsg, nil
}

func (msg BlockMessage) GetBlockInfo() (*ethTypes.Header, uint64, error) {
	if msg.header == nil {
		return nil, 0, fmt.Errorf("header is nil")
	}

	return msg.header, msg.prevBlockTxCount, nil
}

func (msg BlockMessage) MarshalJSON() ([]byte, error) {
	type BlockMessage struct {
		Header           *ethTypes.Header `json:"header"`
		PrevBlockTxCount uint64           `json:"prevBlockTxCount"`
	}

	var enc BlockMessage
	enc.Header = msg.header
	enc.PrevBlockTxCount = msg.prevBlockTxCount

	return json.Marshal(&enc)
}
