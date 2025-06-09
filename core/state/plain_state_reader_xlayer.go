package state

import (
	"bytes"
	"sync"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/dbutils"
	"github.com/ledgerwatch/erigon/core/types/accounts"
)

const (
	DefaultRealtimeCacheSize = 1_000_000
)

// PlainStateCache implements the plain state reader with a changeset cache layer.
// The cache holds a snapshot of the statedb, with a changeset cache layer that
// stores in-memory the state changes.
type PlainStateCache struct {
	snapshotHeight uint64
	snapshotReader *PlainStateReader

	accountLock  sync.RWMutex
	accountCache map[libcommon.Address]*accounts.Account

	storageLock  sync.RWMutex
	storageCache map[string]*uint256.Int

	codeLock  sync.RWMutex
	codeCache map[libcommon.Hash][]byte

	accountIncarnationLock  sync.RWMutex
	accountIncarnationCache map[libcommon.Address]uint64
}

func NewPlainStateCache(db kv.Getter, snapshotHeight uint64) *PlainStateCache {
	return &PlainStateCache{
		snapshotHeight: snapshotHeight,
		snapshotReader: NewPlainStateReader(db),
		accountCache:   make(map[libcommon.Address]*accounts.Account, DefaultRealtimeCacheSize),
		storageCache:   make(map[string]*uint256.Int, DefaultRealtimeCacheSize),
		codeCache:      make(map[libcommon.Hash][]byte, DefaultRealtimeCacheSize),
	}
}

// func (cache *PlainStateCache) ApplyChangeset(changeset *zktypes.Changeset) error {
// 	// // Handle account data changes
// 	// cache.accountLock.Lock()
// 	// for address, balances := range changeset.BalanceChanges {
// 	// 	account, ok := cache.accountCache[address]
// 	// 	if !ok {
// 	// 		account = &accounts.Account{}
// 	// 		cache.accountCache[address] = account
// 	// 	}
// 	// 	account.Balance.Set(balances)
// 	// 	cache.accountCache[address] = account
// 	// 	cache.accountLock.Unlock()
// 	// }
// 	return nil
// }

// func (cache *PlainStateCache) UpdateAccountData(address libcommon.Address, _, account *accounts.Account) error {
// 	cache.accountLock.Lock()
// 	cache.accountCache[address] = account
// 	cache.accountLock.Unlock()

// 	// Override original and assume updating account state incarnation is the latest one
// 	cache.accountIncarnationLock.Lock()
// 	cache.accountIncarnationCache[address] = account.Incarnation
// 	cache.accountIncarnationLock.Unlock()

// 	return nil
// }

// func (cache *PlainStateCache) DeleteAccount(address libcommon.Address, original *accounts.Account) error {
// 	cache.accountLock.Lock()
// 	defer cache.accountLock.Unlock()

// 	// Non-existent / deleted accounts are set to nil
// 	cache.accountCache[address] = nil
// 	return nil
// }

// func (cache *PlainStateCache) WriteAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash, original, value *uint256.Int) error {
// 	cache.storageLock.Lock()
// 	defer cache.storageLock.Unlock()

// 	cache.storageCache[string(dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes()))] = value
// 	return nil
// }

// func (cache *PlainStateCache) CreateContract(codeHash libcommon.Hash, code []byte) error {
// 	cache.codeLock.Lock()
// 	defer cache.codeLock.Unlock()

// 	cache.codeCache[codeHash] = code
// 	return nil
// }

func (cache *PlainStateCache) ReadAccountData(address libcommon.Address) (*accounts.Account, error) {
	cache.accountLock.RLock()
	acc, ok := cache.accountCache[address]
	cache.accountLock.RUnlock()
	if ok {
		return accounts.DeepCopyAccount(acc), nil
	}

	// Cache miss, read from snapshot and load to cache
	acc, err := cache.snapshotReader.ReadAccountData(address)
	if err != nil {
		return nil, err
	}
	return cache.loadAccountFromSnapshot(address, acc), nil
}

func (cache *PlainStateCache) ReadAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash) ([]byte, error) {
	compositeKey := dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())

	cache.storageLock.RLock()
	storage, ok := cache.storageCache[string(compositeKey)]
	cache.storageLock.RUnlock()
	if ok {
		return libcommon.Copy(storage.Bytes()), nil
	}

	// Cache miss, read from snapshot and load to cache
	enc, err := cache.snapshotReader.ReadAccountStorage(address, incarnation, key)
	if err != nil {
		return nil, err
	}
	return cache.loadStorageFromSnapshot(string(compositeKey), enc), nil
}

func (cache *PlainStateCache) ReadAccountCode(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) ([]byte, error) {
	if bytes.Equal(codeHash.Bytes(), emptyCodeHash) {
		return nil, nil
	}

	cache.codeLock.RLock()
	code, ok := cache.codeCache[codeHash]
	cache.codeLock.RUnlock()
	if ok {
		return libcommon.Copy(code), nil
	}

	// Cache miss, read from snapshot and load to cache
	code, err := cache.snapshotReader.ReadAccountCode(address, incarnation, codeHash)
	if err != nil {
		return nil, err
	}
	return cache.loadCodeFromSnapshot(codeHash, code), err
}

func (cache *PlainStateCache) ReadAccountCodeSize(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) (int, error) {
	code, err := cache.ReadAccountCode(address, incarnation, codeHash)
	return len(code), err
}

func (cache *PlainStateCache) ReadAccountIncarnation(address libcommon.Address) (uint64, error) {
	cache.accountIncarnationLock.RLock()
	incarnation, ok := cache.accountIncarnationCache[address]
	cache.accountIncarnationLock.RUnlock()
	if ok {
		return incarnation, nil
	}

	// Cache miss, read from snapshot and load to cache
	incarnation, err := cache.snapshotReader.ReadAccountIncarnation(address)
	if err != nil {
		return 0, err
	}
	return cache.loadAccountIncarnationFromSnapshot(address, incarnation), nil
}

func (cache *PlainStateCache) loadAccountFromSnapshot(address libcommon.Address, accountSnapshot *accounts.Account) *accounts.Account {
	cache.accountLock.Lock()
	defer cache.accountLock.Unlock()
	if account, ok := cache.accountCache[address]; ok {
		// Cache hit
		return accounts.DeepCopyAccount(account)
	}

	// Load to cache
	cache.accountCache[address] = accountSnapshot
	return accountSnapshot
}

func (cache *PlainStateCache) loadStorageFromSnapshot(key string, encSnapshot []byte) []byte {
	cache.storageLock.Lock()
	defer cache.storageLock.Unlock()
	if value, ok := cache.storageCache[key]; ok {
		// Cache hit
		return libcommon.Copy(value.Bytes())
	}

	// Load to cache
	vSnapshot := uint256.NewInt(0).SetBytes(encSnapshot)
	cache.storageCache[key] = vSnapshot
	return encSnapshot
}

func (cache *PlainStateCache) loadCodeFromSnapshot(key libcommon.Hash, codeSnapshot []byte) []byte {
	cache.codeLock.Lock()
	defer cache.codeLock.Unlock()
	if code, ok := cache.codeCache[key]; ok {
		// Cache hit
		return libcommon.Copy(code)
	}

	// Load to cache
	cache.codeCache[key] = codeSnapshot
	return codeSnapshot
}

func (cache *PlainStateCache) loadAccountIncarnationFromSnapshot(address libcommon.Address, incarnationSnapshot uint64) uint64 {
	cache.accountIncarnationLock.Lock()
	defer cache.accountIncarnationLock.Unlock()
	if incarnation, ok := cache.accountIncarnationCache[address]; ok {
		// Cache hit
		return incarnation
	}

	// Load to cache
	cache.accountIncarnationCache[address] = incarnationSnapshot
	return incarnationSnapshot
}
