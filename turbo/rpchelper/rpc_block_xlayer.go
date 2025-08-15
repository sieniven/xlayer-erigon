package rpchelper

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/erigon/zk/hermez_db"
	"github.com/ledgerwatch/erigon/zk/sequencer"
	"github.com/ledgerwatch/erigon/zkevm/jsonrpc/client"
	"github.com/ledgerwatch/erigon/zkevm/log"
)

var (
	// Global sequencer RPC URL for RPC nodes
	// use a global variable to avoid passing the sequencer RPC URL to
	//  every function (which is used in multiple places)
	sequencerRpcUrl string

	// Global variable to store current finalized batch number
	currentFinalizedBatchNumber atomic.Uint64

	// Once to ensure the background goroutine is started only once
	startBackgroundQueryOnce sync.Once
)

// SetSequencerRpcUrl sets the global sequencer RPC URL
func SetSequencerRpcUrl(url string) {
	sequencerRpcUrl = url
}

// GetSequencerRpcUrl returns the global sequencer RPC URL
func GetSequencerRpcUrl() string {
	return sequencerRpcUrl
}

// GetFinalizedBlockNumber returns the finalized block number
// This is a backward-compatible function that uses the global sequencer RPC URL
func GetFinalizedBlockNumber(tx kv.Tx) (uint64, error) {
	return GetFinalizedBlockNumberWithSequencerUrl(tx, GetSequencerRpcUrl())
}

func GetFinalizedBatchNumber(tx kv.Tx) (uint64, error) {
	return getFinalizedBatchNumberWithSequencerUrl(tx, GetSequencerRpcUrl())
}

// GetFinalizedBlockNumberWithSequencerUrl returns the finalized block number
// If running as sequencer, it read from local database
// If running as RPC node, it queries the sequencer for the finalized block header
func GetFinalizedBlockNumberWithSequencerUrl(tx kv.Tx, sequencerRpcUrl string) (uint64, error) {
	if sequencer.IsSequencer() {
		return getFinalizedBlockNumberAsSequencer(tx)
	} else {
		return getFinalizedBlockNumberAsRPC(tx, sequencerRpcUrl)
	}
}

func getFinalizedBatchNumberWithSequencerUrl(tx kv.Tx, sequencerRpcUrl string) (uint64, error) {
	if sequencer.IsSequencer() {
		return getFinalizedBatchNumberAsSequencer(tx)
	} else {
		if bn := currentFinalizedBatchNumber.Load(); bn != 0 {
			return bn, nil
		}
		if sequencerRpcUrl == "" {
			return 0, fmt.Errorf("sequencerRpcUrl is not set")
		}
		bn, err := getFinalizedBatchNumberAsRPC(sequencerRpcUrl)
		if err != nil {
			return 0, err
		}
		currentFinalizedBatchNumber.Store(bn)
		return bn, nil
	}
}

func getFinalizedBatchNumberAsSequencer(tx kv.Tx) (uint64, error) {
	return stages.GetStageProgress(tx, stages.AnalysisGroupVerifiedBatchNo)
}

func getFinalizedBatchNumberAsRPC(sequencerRpcUrl string) (uint64, error) {
	if sequencerRpcUrl == "" {
		return 0, fmt.Errorf("sequencerRpcUrl is not set")
	}

	response, err := client.JSONRPCCall(sequencerRpcUrl, "zkevm_finalizedBatchNumber")
	if err != nil {
		return 0, fmt.Errorf("failed to call zkevm_finalizedBatchNumber to sequencer.err:%v. sequencerRpcUrl:%s", err, sequencerRpcUrl)
	}
	return transHexToUint64(response.Result)
}

// getFinalizedBlockNumberAsSequencer implements the original logic for sequencer nodes
func getFinalizedBlockNumberAsSequencer(tx kv.Tx) (uint64, error) {
	// get highest verified batch
	highestVerifiedBatchNo, err := stages.GetStageProgress(tx, stages.AnalysisGroupVerifiedBatchNo)
	if err != nil {
		return 0, err
	}
	hermezDb := hermez_db.NewHermezDbReader(tx)
	// we've got the highest batch to execute to, now get it's highest block
	highestVerifiedBlockHeight, _, err := hermezDb.GetHighestBlockInBatch(highestVerifiedBatchNo)
	if err != nil {
		return 0, err
	}

	var highestBlockNumber uint64
	highestBlockNumber, err = stages.GetStageProgress(tx, stages.Execution)
	if err != nil {
		return 0, fmt.Errorf("getting latest finished block number: %w", err)
	}

	blockNumber := highestVerifiedBlockHeight
	if highestBlockNumber < blockNumber {
		blockNumber = highestBlockNumber
	}

	return blockNumber, nil
}

// getFinalizedBlockNumberAsRPC implements the logic for RPC nodes by querying the sequencer
func getFinalizedBlockNumberAsRPC(tx kv.Tx, sequencerRpcUrl string) (uint64, error) {
	if sequencerRpcUrl == "" {
		panic("sequencerRpcUrl is not set")
	}

	// Query the sequencer for the finalized block header
	blockNumber, err := querySequencerForFinalizedBlock(tx, sequencerRpcUrl)
	if err != nil {
		return 0, fmt.Errorf("failed to query sequencer for finalized block: %w", err)
	}

	return blockNumber, nil
}

// querySequencerForFinalizedBlock gets the finalized block number from the current finalized batch number
func querySequencerForFinalizedBlock(tx kv.Tx, sequencerRpcUrl string) (uint64, error) {
	if sequencerRpcUrl == "" {
		return 0, fmt.Errorf("sequencerRpcUrl is not set")
	}
	// Read current finalized batch number from the poller cache
	batchNumber := currentFinalizedBatchNumber.Load()
	if batchNumber == 0 {
		// Fallback once to avoid empty result before poller kicks in
		bn, err := getFinalizedBatchNumberAsRPC(sequencerRpcUrl)
		if err != nil {
			return 0, fmt.Errorf("no finalized batch number available yet: %w", err)
		}
		currentFinalizedBatchNumber.Store(bn)
		batchNumber = bn
	}

	// Use hermez database to get the highest block in the finalized batch
	hermezDb := hermez_db.NewHermezDbReader(tx)
	blockNumber, _, err := hermezDb.GetHighestBlockInBatch(batchNumber)
	if err != nil {
		return 0, fmt.Errorf("failed to get highest block in batch %d: %w", batchNumber, err)
	}

	return blockNumber, nil
}

func transHexToUint64(hex json.RawMessage) (uint64, error) {
	var result string
	err := json.Unmarshal(hex, &result)
	if err != nil {
		return 0, err
	}

	if len(result) > 1 && (result[:2] == "0x" || result[:2] == "0X") {
		result = result[2:]
	}

	result1, err := strconv.ParseUint(result, 16, 64)
	if err != nil {
		return 0, err
	}

	return result1, nil
}

// StartFinalizedBatchPoller starts a background goroutine that queries the sequencer
// for finalized batch number on given interval; stops when ctx is done.
func StartFinalizedBatchPoller(ctx context.Context, interval time.Duration) {
	if sequencer.IsSequencer() {
		return
	}
	startBackgroundQueryOnce.Do(func() {
		go startBackgroundQuery(ctx, interval)
	})
}

// startBackgroundQuery starts a background goroutine that queries the sequencer
// for finalized batch number every second
func startBackgroundQuery(ctx context.Context, interval time.Duration) {
	if sequencerRpcUrl == "" {
		log.Warn("finalized batch poller not started: empty sequencer RPC URL")
		return
	}
	// initial fetch
	if bn, err := getFinalizedBatchNumberAsRPC(sequencerRpcUrl); err != nil {
		log.Error("failed to get finalized batch number from sequencer (initial)", "err", err)
	} else {
		currentFinalizedBatchNumber.Store(bn)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("finalized batch poller stopped")
			return
		case <-ticker.C:
			bn, err := getFinalizedBatchNumberAsRPC(sequencerRpcUrl)
			if err != nil {
				log.Error("failed to get finalized batch number from sequencer", "err", err)
				continue
			}
			currentFinalizedBatchNumber.Store(bn)
		}
	}
}
