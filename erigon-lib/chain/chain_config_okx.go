package chain

import "github.com/ledgerwatch/erigon-lib/common"

// TokenManagerConfig Token Manager precompile配置
type TokenManagerConfig struct {
	BurnAuthorizedAddresses []common.Address `json:"burnAuthorizedAddresses,omitempty"`
	ActivationBlock         *uint64          `json:"activationBlock,omitempty"` // Block height when Token Manager becomes active
}
