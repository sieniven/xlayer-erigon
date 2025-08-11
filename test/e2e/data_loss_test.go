//go:build !skip_smoke
// +build !skip_smoke

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	kv2 "github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/ethclient"
	smtDb "github.com/ledgerwatch/erigon/smt/pkg/db"
	"github.com/ledgerwatch/erigon/test/operations"
	"github.com/ledgerwatch/erigon/zk/hermez_db"
	"github.com/ledgerwatch/erigon/zkevm/encoding"
	"github.com/ledgerwatch/erigon/zkevm/log"
	elog "github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"
)

// To run this test, please refer to docs/data-loss-case.md
// Warning: we should compare the tx nonce, block hash before and after the data loss, they should be the same
const nonceFile = "../data/nonce.txt"
const blockHashFile = "../data/block_hash.txt"
const maxLoop = 10000
const stopBatch = 11

// before doFinishBlockAndUpdateState (before RemoveMinedTransactions)
// txs are still in txpool, data is lost in chaindata and not written to datastream yet
func TestModifyCodeCase1(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	// Check if 'os' import is already present, if not add it
	if !strings.Contains(content, "\"os\"") {
		importIndex := strings.Index(content, "import (")
		if importIndex != -1 {
			content = content[:importIndex+8] + "\n\t\"os\"" + content[importIndex+8:]
		}
	}

	// Insert lose data logic before doFinishBlockAndUpdateState
	blockToInsert := `
// For data loss
if batchState.batchNumber == 11 {
	log.Warn(fmt.Sprintf("Waring Stop before doFinishBlockAndUpdateState:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("doFinishBlockAndUpdateState:%v,%v", batchState.batchNumber, blockNumber))`

	lines := strings.Split(content, "\n")
	inserted := false
	for i, line := range lines {
		if strings.Contains(line, "doFinishBlockAndUpdateState") {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted before doFinishBlockAndUpdateState")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

// before 1st CommitAndStart (before RemoveMinedTransactions)
// txs are still in txpool, data is lost in chaindata and not written to datastream yet
func TestModifyCodeCase2(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	// Check if 'os' import is already present, if not add it
	if !strings.Contains(content, "\"os\"") {
		importIndex := strings.Index(content, "import (")
		if importIndex != -1 {
			content = content[:importIndex+8] + "\n\t\"os\"" + content[importIndex+8:]
		}
	}

	// Insert lose data logic before the 1st CommitAndStart
	blockToInsert := `
// For data loss
if batchState.batchNumber == 11 {
	log.Warn(fmt.Sprintf("Stop before the 1st CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("1st CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`

	lines := strings.Split(content, "\n")
	inserted := false
	count := 0
	for i, line := range lines {
		if strings.Contains(line, "sdb.CommitAndStart()") {
			count++
		}
		if count == 1 && !inserted {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted before the 1st CommitAndStart")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

// before 2nd CommitAndStart (after RemoveMinedTransactions and updateStreamAndCheckRollback)
// txs are removed from txpool, data is lost in chaindata
// assuming current block having been written to ds
func TestModifyCodeCase3(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	// Check if 'os' import is already present, if not add it
	if !strings.Contains(content, "\"os\"") {
		importIndex := strings.Index(content, "import (")
		if importIndex != -1 {
			content = content[:importIndex+8] + "\n\t\"os\"" + content[importIndex+8:]
		}
	}

	// Insert lose data logic before the 2nd CommitAndStart
	blockToInsert := `
// For data loss
if batchState.batchNumber == 11 {
	log.Warn(fmt.Sprintf("Stop before the 2nd CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("2nd CommitAndStart:%v,%v", batchState.batchNumber, blockNumber))`

	lines := strings.Split(content, "\n")
	inserted := false
	count := 0
	for i, line := range lines {
		if strings.Contains(line, "sdb.CommitAndStart()") {
			count++
		}
		if count == 2 && !inserted {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted before the 2nd CommitAndStart")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

// before the last sdb.tx.Commit (after RemoveMinedTransactions and updateStreamAndCheckRollback)
// txs are removed from txpool, data is lost in chaindata
// assuming current block having been written to ds
func TestModifyCodeCase4(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	// Check if 'os' import is already present, if not add it
	if !strings.Contains(content, "\"os\"") {
		importIndex := strings.Index(content, "import (")
		if importIndex != -1 {
			content = content[:importIndex+8] + "\n\t\"os\"" + content[importIndex+8:]
		}
	}

	// Insert lose data logic after the 'Finish batch' log
	blockToInsert := `
// For data loss
if batchState.batchNumber == 11 {
	log.Warn(fmt.Sprintf("Stop before the last sdb.tx.Commit():%v,%v", batchState.batchNumber, block.Number))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("last sdb.tx.Commit():%v,%v", batchState.batchNumber, block.Number))`

	lines := strings.Split(content, "\n")
	inserted := false
	for i, line := range lines {
		if strings.Contains(line, "Finish batch") {
			lines = append(lines[:i+1], append([]string{blockToInsert}, lines[i+1:]...)...)
			inserted = true
			break
		}
	}

	require.True(t, inserted, "Expected the block to be inserted after the 'Finish batch' log")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

// between RemoveMinedTransactions and updateStreamAndCheckRollback
// txs are removed from txpool, data is not written to datastream yet
func TestModifyCodeCase5(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	// Check if 'os' import is already present, if not add it
	if !strings.Contains(content, "\"os\"") {
		importIndex := strings.Index(content, "import (")
		if importIndex != -1 {
			content = content[:importIndex+8] + "\n\t\"os\"" + content[importIndex+8:]
		}
	}

	// Insert lose data logic before WriteBlockDetailsToDatastream
	blockToInsert := `
// For data loss
if batchState.batchNumber == 11 {
	log.Warn(fmt.Sprintf("Stop before WriteBlockDetailsToDatastream:%v,%v", batchState.batchNumber, block.Number))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("WriteBlockDetailsToDatastream:%v,%v", batchState.batchNumber, block.Number))`

	lines := strings.Split(content, "\n")
	inserted := false
	for i, line := range lines {
		if strings.Contains(line, "streamWriter.WriteBlockDetailsToDatastream(") {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}
	require.True(t, inserted, "Expected the block to be inserted before streamWriter.WriteBlockDetailsToDatastream")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

// between doFinishBlockAndUpdateState and SetSmtCache
// stage.Execution is updated, blockCache hasn't been set, data is not written to datastream yet
func TestModifyCodeCase8(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	// Check if 'os' import is already present, if not add it
	if !strings.Contains(content, "\"os\"") {
		importIndex := strings.Index(content, "import (")
		if importIndex != -1 {
			content = content[:importIndex+8] + "\n\t\"os\"" + content[importIndex+8:]
		}
	}

	// Insert lose data logic before SetSmtCache
	blockToInsert := `
// For data loss
if batchState.batchNumber == 11 {
	log.Warn(fmt.Sprintf("Stop before SetSmtCache:%v,%v", batchState.batchNumber, block.Number))
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
log.Info(fmt.Sprintf("SetSmtCache:%v,%v", batchState.batchNumber, block.Number))`

	lines := strings.Split(content, "\n")
	inserted := false
	for i, line := range lines {
		if strings.Contains(line, "s.SetSmtCache(blockNumber, blockCache)") {
			lines = append(lines[:i], append([]string{blockToInsert}, lines[i:]...)...)
			inserted = true
			break
		}
	}
	require.True(t, inserted, "Expected the block to be inserted before SetSmtCache")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

// comment handleShutdown, disable the graceful shutdown
func TestModifyCodeCase6(t *testing.T) {
	// File to modify
	filePath := "../../turbo/stages/stageloop_xlayer.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	blockToReplace := `
// For data loss
// handleShutdown(ctx, s, config, &wg, workerPool, db, cache, logger)`

	lines := strings.Split(content, "\n")
	replaced := false
	for i, line := range lines {
		if strings.Contains(line, "handleShutdown(") {
			lines[i] = blockToReplace
			replaced = true
			break
		}
	}
	require.True(t, replaced, "Expected to find and replace the handleShutdown line")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

// comment smt alignment, disable the auto recovery
func TestModifyCodeCase7(t *testing.T) {
	// File to modify
	filePath := "../../zk/stages/stage_sequence_execute_xlayer.go"

	// Read the file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading file:", err)
	}

	// Convert data to string for easier manipulation
	content := string(data)

	blockToReplace := `
// For data loss
var shouldCheckForExecutionAndSMTAlignment = SMTAlignmentTerminated`

	lines := strings.Split(content, "\n")
	replaced := false
	for i, line := range lines {
		if strings.Contains(line, "var shouldCheckForExecutionAndSMTAlignment = SMTAlignmentInit") {
			lines[i] = blockToReplace
			replaced = true
			break
		}
	}
	require.True(t, replaced, "Expected to find and replace the shouldCheckForExecutionAndSMTAlignment line")

	content = strings.Join(lines, "\n")

	// Write the modified content back to the file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal("Error writing file:", err)
	}
}

func TestStressAndStopSeq(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	for i := 1; i < maxLoop; i++ {
		from := common.HexToAddress(operations.DefaultL2AdminAddress)
		to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
		nonce, err := client.PendingNonceAt(ctx, from)
		require.NoError(t, err)
		var tx types.Transaction = &types.LegacyTx{
			CommonTx: types.CommonTx{
				Nonce: nonce,
				To:    &to,
				Gas:   21000,
				Value: uint256.NewInt(0),
			},
			GasPrice: uint256.NewInt(uint64(i) * 10 * encoding.Gwei),
		}
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
		require.NoError(t, err)
		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
		signedTx, err := types.SignTx(tx, *signer, privateKey)
		require.NoError(t, err)
		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)
		batchNum, err := operations.GetBatchNumber()
		block, err := operations.GetBlockNumber()
		require.NoError(t, err)
		blockHash, err := operations.GetBlockHashByNumber(block)
		require.NoError(t, err)
		if nonce%100 == 0 {
			log.Info(fmt.Sprintf("Cur Batch Number: %v, nonce :%v", batchNum, nonce))
		}
		if batchNum == uint64(stopBatch-1) {
			log.Info(fmt.Sprintf("Stop before write nonce: %v", nonce))
			err = writeNonce(nonce)
			err = writeBlockHash(block, blockHash)
			require.NoError(t, err)
			break
		}
	}

	require.NoError(t, err)

	batchNum, err := operations.GetBatchNumber()
	require.NoError(t, err)
	require.Equal(t, batchNum, uint64(stopBatch-1))
}

func TestCheckVerify(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	auth, err := operations.GetAuth(operations.DefaultL2AdminPrivateKey, operations.DefaultL2ChainID)
	require.NoError(t, err)
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	from := common.HexToAddress(operations.DefaultL2AdminAddress)
	to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	var nonce uint64
	nonce, err = client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	var rNonce uint64
	rNonce, err = readNonce()
	require.NoError(t, err)
	log.Info(fmt.Sprintf("pending nonce: %v, read nonce: %v", nonce, rNonce))
	require.Equal(t, nonce, rNonce+1)

	// check block hash
	blockRead, blockHashRead, err := readBlockHash()
	require.NoError(t, err)
	blockHash, err := operations.GetBlockHashByNumber(blockRead)
	require.NoError(t, err)
	require.Equal(t, blockHash, blockHashRead)

	var tx types.Transaction = &types.LegacyTx{
		CommonTx: types.CommonTx{
			Nonce: nonce,
			To:    &to,
			Gas:   21000,
			Value: uint256.NewInt(0),
		},
		GasPrice: uint256.NewInt(10 * encoding.Gwei),
	}
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), 1, 0)
	signedTx, err := types.SignTx(tx, *signer, privateKey)
	require.NoError(t, err)
	var txs []*types.Transaction
	txs = append(txs, &signedTx)
	log.Info(fmt.Sprintf("signedTx nonce: %v", signedTx.GetNonce()))
	_, err = operations.ApplyL2Txs(ctx, txs, auth, client, operations.VerifiedConfirmationLevel)
	require.NoError(t, err)
	err = writeNonce(nonce)
	require.NoError(t, err)
}

// Verify SMT root equals block header root for latest finished block
func TestVerifySmtRootMatchesHeaderRoot(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()

	// resolve host data directories (relative to test/e2e)
	chainDir := filepath.Clean("../data/seq/chaindata")
	smtDir := filepath.Clean("../data/seq/smt")

	logger := elog.New()

	// open chain db (always exists)
	chainDB, err := kv2.NewMDBX(logger).Path(chainDir).Open(ctx)
	require.NoError(t, err)
	defer chainDB.Close()

	// transactions will be opened inside the loop each iteration

	// construct read-only SMT DB wrapper
	// We'll open fresh txs inside the retry loop to avoid stale snapshots.

	deadline := time.Now().Add(180 * time.Second)
	for {
		chainTx, err := chainDB.BeginRo(ctx)
		if err != nil {
			if time.Now().After(deadline) {
				require.NoError(t, err)
			}
			time.Sleep(1 * time.Second)
			continue
		}
		// prefer latest closed batch end block to avoid racing with head
		var headerRoot common.Hash
		if closedBlk, err := hermez_db.NewHermezDbReader(chainTx).GetLatestBatchEndBlock(); err == nil && closedBlk > 0 {
			if hdr := rawdb.ReadHeaderByNumber(chainTx, closedBlk); hdr != nil {
				headerRoot = hdr.Root
			}
		} else {
			if head, _ := rawdb.ReadLastBlockSynced(chainTx); head != nil {
				headerRoot = head.Root()
			}
		}

		// read SMT last root, prefer standalone SMT DB if present; otherwise fall back to chainDB
		var smtRoot common.Hash
		smtDat := filepath.Join(smtDir, "mdbx.dat")
		if info, statErr := os.Stat(smtDat); statErr == nil && info.Size() > 0 {
			// sequencer has been stopped in make, so it's safe to open host SMT DB
			smtDB, openErr := kv2.NewMDBX(logger).Path(smtDir).Open(ctx)
			require.NoError(t, openErr)
			smtTx, txErr := smtDB.BeginRo(ctx)
			require.NoError(t, txErr)
			ro := smtDb.NewRoEriDb(smtTx, chainTx)
			lastRootBig, getErr := ro.GetLastRoot()
			require.NoError(t, getErr)
			smtRoot = common.BigToHash(lastRootBig)
			smtTx.Rollback()
			smtDB.Close()
		} else {
			ro := smtDb.NewRoEriDb(chainTx, chainTx)
			lastRootBig, getErr := ro.GetLastRoot()
			require.NoError(t, getErr)
			smtRoot = common.BigToHash(lastRootBig)
		}

		chainTx.Rollback()

		if smtRoot == headerRoot {
			log.Info(fmt.Sprintf("SMT root matches header root: %s", smtRoot.Hex()))
			break
		}

		if time.Now().After(deadline) {
			require.Equal(t, headerRoot, smtRoot, "timeout waiting for SMT root to match header root")
		}
		time.Sleep(1 * time.Second)
	}

}

func writeNonce(nonce uint64) error {
	data := strconv.FormatUint(nonce, 10)
	return os.WriteFile(nonceFile, []byte(data), 0644)
}

func readNonce() (uint64, error) {
	data, err := os.ReadFile(nonceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0, nil
	}

	nonce, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, err
	}

	return nonce, nil
}

func writeBlockHash(block uint64, blockHash string) error {
	log.Info(fmt.Sprintf("writeBlockHash, block: %d, blockHash: %s", block, blockHash))
	data := fmt.Sprintf("%d,%s", block, blockHash)
	return os.WriteFile(blockHashFile, []byte(data), 0644)
}

func readBlockHash() (uint64, string, error) {
	data, err := os.ReadFile(blockHashFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, "", nil
		}
		return 0, "", err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0, "", nil
	}
	parts := strings.Split(trimmed, ",")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid block hash file format")
	}
	block, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return 0, "", err
	}
	log.Info(fmt.Sprintf("readBlockHash, block: %d, blockHash: %s", block, parts[1]))
	return block, parts[1], nil
}
