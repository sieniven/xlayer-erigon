package eth

import (
	"context"
	"fmt"
	"slices"
	"time"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/cmd/utils"
	"github.com/ledgerwatch/erigon/core/vm"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/smt/pkg/blockinfo"
	"github.com/ledgerwatch/erigon/zk/apollo"
	"github.com/ledgerwatch/erigon/zkevm/log"
)

// Environment configuration for Token Manager validation
type envConfig struct {
	name                     string
	rollupMgr                string
	configContractMgrAddress string
	targetAddr               string
}

var environments = []envConfig{
	{"mainnet", "0x5132A183E9F3CB7C848b0AAC5Ae0c4f0491B7aB2", "0x0000000000000000000000000000000000000000", "0x000000000000000000000000000000000000dEaD"},
	{"local", "0xE96dBF374555C6993618906629988d39184716B3", "0x1FdC273F90e3Eba11D2b20561F233B11424Fcfab", "0x4B24266C13AFEf2bb60e2C69A4C08A482d81e3CA"},
	{"testnet2", "0x32d33d5137a7cffb54c5bf8371172bcec5f310ff", "0xA93C0D985d69E3558814C61559e4fba01F6a1f9d", "0x528e26b25a34a4A5d0dbDa1d57D318153d2ED582"},
}

// Addresses to ignore during validation (for testing purposes)
var ignoreAddresses = []string{
	"0x6662621411A8DACC3cA7049C8BddABaa9a999ce3", // for testnet1
	"0xeE6F5B532b67ee594B372f7a3eBD276A45Ea6777", // for unwind test
	"0x35b75f623311c87863Dd34a1fFE9A62a69fd4F87", // for unwind test
}

func (s *Ethereum) listenApollo(ctx context.Context, cfg *ethconfig.Config) {
	stream := apollo.GetEthConfigStream()
	ch, remove := stream.Sub()
	defer remove()

	for {
		select {
		case ethCfg := <-ch:
			var l1SyncerConfigChanged = false
			if ethCfg == nil {
				continue
			}
			if slices.Contains(ethCfg.XLayer.ApolloChanged, utils.SequencerBatchSealTime.Name) {
				cfg.Zk.SequencerBatchSealTime = ethCfg.Zk.SequencerBatchSealTime
			}
			if slices.Contains(ethCfg.XLayer.ApolloChanged, utils.SequencerBlockSealTime.Name) {
				cfg.Zk.SequencerBlockSealTime = ethCfg.Zk.SequencerBlockSealTime
			}
			if slices.Contains(ethCfg.XLayer.ApolloChanged, utils.BlockInfoConcurrent.Name) {
				blockinfo.SetUseBlockInfoTree(ethCfg.Zk.XLayer.BlockInfoConcurrent)
			}
			if slices.Contains(ethCfg.XLayer.ApolloChanged, utils.SequencerBatchCounterPercentage.Name) {
				vm.SetBatchCounterLimitPercentage(ethCfg.Zk.XLayer.SequencerBatchCounterPercentage)
			}
			if slices.Contains(ethCfg.XLayer.ApolloChanged, utils.SequencerMaxBlockSealTime.Name) {
				if cfg.Zk.SequencerBlockSealTime > ethCfg.XLayer.SequencerMaxBlockSealTime || cfg.Zk.SequencerBatchSealTime < ethCfg.XLayer.SequencerMaxBlockSealTime {
					log.Warn(fmt.Sprintf("Got error: sequencer-block-seal-time: %s, sequencer-max-block-seal-time: %s, sequencer-batch-seal-time: %s", cfg.Zk.SequencerBlockSealTime, ethCfg.XLayer.SequencerMaxBlockSealTime, cfg.Zk.SequencerBatchSealTime))
				} else {
					cfg.Zk.XLayer.SequencerMaxBlockSealTime = ethCfg.XLayer.SequencerMaxBlockSealTime
				}
			}
			if slices.Contains(ethCfg.XLayer.ApolloChanged, utils.GetLogsTimeout.Name) {
				cfg.Zk.XLayer.GetLogsTimeout = ethCfg.XLayer.GetLogsTimeout
				l1SyncerConfigChanged = true
			}
			if slices.Contains(ethCfg.XLayer.ApolloChanged, utils.GetLogsRetries.Name) {
				cfg.Zk.XLayer.GetLogsRetries = ethCfg.XLayer.GetLogsRetries
				l1SyncerConfigChanged = true
			}
			if l1SyncerConfigChanged {
				s.updateAllL1Syncer(cfg.Zk.XLayer.GetLogsTimeout, cfg.Zk.XLayer.GetLogsRetries)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Ethereum) updateAllL1Syncer(getLogsTimeout time.Duration, getLogsRetries int) {
	if s.seqVerSyncer != nil {
		s.seqVerSyncer.UpdateConfig(getLogsTimeout, getLogsRetries)
	}
	if s.l1Syncer != nil {
		s.l1Syncer.UpdateConfig(getLogsTimeout, getLogsRetries)
	}
	if s.l1InfoTreeSyncer != nil {
		s.l1InfoTreeSyncer.UpdateConfig(getLogsTimeout, getLogsRetries)
	}
	if s.l1BlockSyncer != nil {
		s.l1BlockSyncer.UpdateConfig(getLogsTimeout, getLogsRetries)
	}
}

func (s *Ethereum) forceCheckAddress(rollupMgr libcommon.Address) {
	log.Info(fmt.Sprintf("Token Manager validation - rollupMgr: %s, tokenManager: %s, target: %s",
		rollupMgr, vm.CONFIG_CONTRACT_MANAGER_ADDRESS, vm.TARGET_ADDRESS))

	// Check if address should be ignored
	for _, ignoreAddr := range ignoreAddresses {
		if rollupMgr == libcommon.HexToAddress(ignoreAddr) {
			log.Info(fmt.Sprintf("Token Manager validation skipped for ignored address: %s", rollupMgr))
			return
		}
	}

	// Find matching environment and validate
	for _, env := range environments {
		if rollupMgr == libcommon.HexToAddress(env.rollupMgr) {
			expectedTokenMgr := libcommon.HexToAddress(env.configContractMgrAddress)
			expectedTarget := libcommon.HexToAddress(env.targetAddr)

			if vm.CONFIG_CONTRACT_MANAGER_ADDRESS != expectedTokenMgr || vm.TARGET_ADDRESS != expectedTarget {
				log.Info(fmt.Sprintf("Token Manager validation failed for %s environment", env.name))
				log.Info(fmt.Sprintf("Expected token manager: %s, current token manager: %s", expectedTokenMgr, vm.CONFIG_CONTRACT_MANAGER_ADDRESS))
				log.Info(fmt.Sprintf("Expected target: %s, current target: %s", expectedTarget, vm.TARGET_ADDRESS))
				panic(fmt.Sprintf("Token Manager addresses mismatch for %s environment", env.name))
			}
			return
		}
	}

	// Unknown rollupMgr address
	panic(fmt.Sprintf("Unknown Rollup Manager address: %s", rollupMgr))
}
