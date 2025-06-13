package realtime

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/dbutils"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/types/accounts"
	"github.com/ledgerwatch/erigon/crypto"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/erigon/turbo/trie"
	realtimeTypes "github.com/ledgerwatch/erigon/zk/realtime/types"
	"github.com/ledgerwatch/erigon/zkevm/log"
)

var (
	ErrNotReady   = fmt.Errorf("state cache not initialized")
	emptyCodeHash = crypto.Keccak256(nil)
)

// PlainStateCache implements the plain state reader with a changeset cache layer.
// The cache holds a snapshot of the statedb, with a changeset cache layer that
// stores in-memory the state changes.
type PlainStateCache struct {
	ctx            context.Context
	db             kv.RoDB
	tx             kv.Tx
	snapshotReader *state.PlainStateReader
	snapshotHeight atomic.Uint64

	cacheLock        sync.RWMutex
	accountCache     map[libcommon.Address]*accounts.Account
	storageCache     map[string]*uint256.Int
	codeCache        map[libcommon.Hash][]byte
	incarnationCache map[libcommon.Address]uint64
}

func NewPlainStateCache(ctx context.Context, db kv.RoDB, size int) (*PlainStateCache, error) {
	return &PlainStateCache{
		ctx:              ctx,
		db:               db,
		tx:               nil,
		snapshotReader:   nil,
		snapshotHeight:   atomic.Uint64{},
		accountCache:     make(map[libcommon.Address]*accounts.Account, size),
		storageCache:     make(map[string]*uint256.Int, size),
		codeCache:        make(map[libcommon.Hash][]byte, size),
		incarnationCache: make(map[libcommon.Address]uint64, size),
	}, nil
}

func (cache *PlainStateCache) InitSnapshotReader() error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	if cache.snapshotReader != nil || cache.tx != nil {
		return fmt.Errorf("Snapshot reader already initialized")
	}

	var err error
	cache.tx, err = cache.db.BeginRo(cache.ctx)
	if err != nil {
		return err
	}
	cache.snapshotReader = state.NewPlainStateReader(cache.tx)

	blockNum, err := stages.GetStageProgress(cache.tx, stages.Finish)
	if err != nil {
		return err
	}
	cache.snapshotHeight.Store(blockNum)

	return nil
}

func (cache *PlainStateCache) Clear() {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Clear all caches
	for k := range cache.accountCache {
		delete(cache.accountCache, k)
	}
	for k := range cache.storageCache {
		delete(cache.storageCache, k)
	}
	for k := range cache.codeCache {
		delete(cache.codeCache, k)
	}
	for k := range cache.incarnationCache {
		delete(cache.incarnationCache, k)
	}

	// Rollback ro-tx and clear snapshot reader
	if cache.tx != nil {
		cache.tx.Rollback()
		cache.tx = nil
		cache.snapshotReader = nil
	}
	cache.snapshotHeight.Store(0)
}

func (cache *PlainStateCache) ApplyChangeset(changeset *realtimeTypes.Changeset, blockNumber uint64, txIndex uint) error {
	// Handle account data changes
	addressChanges := make(map[libcommon.Address]*accounts.Account)
	cache.applyChangesetToAccountData(changeset, addressChanges)

	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Apply code changes
	for address, code := range changeset.CodeChanges {
		account, ok := addressChanges[address]
		if !ok {
			return fmt.Errorf("apply code changes failed: no codehash received")
		}
		cache.codeCache[account.CodeHash] = code
	}

	// Apply storage changes
	for address, storage := range changeset.StorageChanges {
		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply storage changes failed: %v", err)
		}

		for key, value := range storage {
			compositeKey := dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), account.Incarnation, key.Bytes())
			cache.storageCache[string(compositeKey)] = value
		}
	}

	// Apply deleted accounts changes
	for address := range changeset.DeletedAccounts {
		// Non-existent / deleted accounts are set to nil
		addressChanges[address] = nil
	}

	// Apply account changes
	for address, account := range addressChanges {
		delete(cache.accountCache, address)
		cache.accountCache[address] = account
		log.Info("ApplyChangeset: ", address)
	}

	log.Info(fmt.Sprintf("Apply changeset from tx with height: %d, txIndex: %d\n", blockNumber, txIndex))

	return nil
}

func (cache *PlainStateCache) applyChangesetToAccountData(changeset *realtimeTypes.Changeset, addressChanges map[libcommon.Address]*accounts.Account) (err error) {
	// Apply balance changes
	for address, balance := range changeset.BalanceChanges {
		if _, ok := changeset.DeletedAccounts[address]; ok {
			continue
		}

		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply balance changes failed: %v", err)
		}
		account.Balance.Set(balance)
	}

	// Apply nonce changes
	for address, nonce := range changeset.NonceChanges {
		if _, ok := changeset.DeletedAccounts[address]; ok {
			continue
		}

		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply nonce changes failed: %v", err)
		}
		account.Nonce = nonce
	}

	// Apply code hash changes
	for address, codeHash := range changeset.CodeHashChanges {
		if _, ok := changeset.DeletedAccounts[address]; ok {
			continue
		}

		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply code hash changes failed: %v", err)
		}
		account.CodeHash = codeHash
	}

	// Apply incarnation changes
	for address, incarnation := range changeset.IncarnationChanges {
		cache.incarnationCache[address] = incarnation

		if _, ok := changeset.DeletedAccounts[address]; ok {
			continue
		}

		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply incarnation changes failed: %v", err)
		}
		account.PrevIncarnation = incarnation - 1
		account.Incarnation = incarnation
	}

	return nil
}

func (cache *PlainStateCache) ReadAccountData(address libcommon.Address) (*accounts.Account, error) {
	if cache.snapshotReader == nil {
		return nil, ErrNotReady
	}

	cache.cacheLock.RLock()
	acc, ok := cache.accountCache[address]
	if ok {
		accCopy := accounts.DeepCopyAccount(acc)
		cache.cacheLock.RUnlock()
		return accCopy, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountData(address)
}

func (cache *PlainStateCache) ReadAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash) ([]byte, error) {
	if cache.snapshotReader == nil {
		return nil, ErrNotReady
	}

	compositeKey := dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())

	cache.cacheLock.RLock()
	storage, ok := cache.storageCache[string(compositeKey)]
	if ok {
		storageCopy := libcommon.Copy(storage.Bytes())
		cache.cacheLock.RUnlock()
		return storageCopy, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountStorage(address, incarnation, key)
}

func (cache *PlainStateCache) ReadAccountCode(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) ([]byte, error) {
	if bytes.Equal(codeHash.Bytes(), emptyCodeHash) {
		return nil, nil
	}

	if cache.snapshotReader == nil {
		return nil, ErrNotReady
	}

	cache.cacheLock.RLock()
	code, ok := cache.codeCache[codeHash]
	if ok {
		codeCopy := libcommon.Copy(code)
		cache.cacheLock.RUnlock()
		return codeCopy, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountCode(address, incarnation, codeHash)
}

func (cache *PlainStateCache) ReadAccountCodeSize(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) (int, error) {
	code, err := cache.ReadAccountCode(address, incarnation, codeHash)
	return len(code), err
}

func (cache *PlainStateCache) ReadAccountIncarnation(address libcommon.Address) (uint64, error) {
	if cache.snapshotReader == nil {
		return 0, ErrNotReady
	}

	cache.cacheLock.RLock()
	incarnation, ok := cache.incarnationCache[address]
	cache.cacheLock.RUnlock()
	if ok {
		return incarnation, nil
	}

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountIncarnation(address)
}

func (cache *PlainStateCache) unsafeReadAccountData(address libcommon.Address) (*accounts.Account, error) {
	acc, ok := cache.accountCache[address]
	if ok {
		return acc, nil
	}

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountData(address)
}

func (cache *PlainStateCache) getOrCreateAccount(address libcommon.Address, addressChanges map[libcommon.Address]*accounts.Account) (*accounts.Account, error) {
	account, ok := addressChanges[address]
	if !ok {
		var err error
		account, err = cache.unsafeReadAccountData(address)
		if err != nil {
			return nil, err
		}

		if account == nil {
			// Non-existent account, create new account
			account, err = cache.createAccount()
			if err != nil {
				return nil, err
			}
		}
		addressChanges[address] = account
	}

	return account, nil
}

func (cache *PlainStateCache) createAccount() (*accounts.Account, error) {
	return &accounts.Account{
		Initialised: true,
		Root:        libcommon.BytesToHash(trie.EmptyRoot[:]),
		CodeHash:    libcommon.BytesToHash(emptyCodeHash),
	}, nil
}

func (cache *PlainStateCache) GetSnapshotHeight() uint64 {
	return cache.snapshotHeight.Load()
}
