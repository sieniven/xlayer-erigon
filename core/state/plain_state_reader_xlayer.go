package state

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/dbutils"
	"github.com/ledgerwatch/erigon/core/types/accounts"
)

const (
	DefaultRealtimeCacheSize = 1_000_000
)

// PlainStateCache implements the plain state reader and writer. The cache holds a snapshot
// of the statedb, with a cache layer that stores in-memory the state changeset.
type PlainStateCache struct {
	db             kv.Getter
	snapshotHeight uint64

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
		db:             db,
		snapshotHeight: snapshotHeight,
		accountCache:   make(map[libcommon.Address]*accounts.Account, DefaultRealtimeCacheSize),
		storageCache:   make(map[string]*uint256.Int, DefaultRealtimeCacheSize),
		codeCache:      make(map[libcommon.Hash][]byte, DefaultRealtimeCacheSize),
	}
}

func (cache *PlainStateCache) UpdateAccountData(address common.Address, _, account *accounts.Account) error {
	cache.accountLock.Lock()
	cache.accountCache[address] = account
	cache.accountLock.Unlock()

	// Override original and assume updating account state incarnation is the latest one
	cache.accountIncarnationLock.Lock()
	cache.accountIncarnationCache[address] = account.Incarnation
	cache.accountIncarnationLock.Unlock()

	return nil
}

func (cache *PlainStateCache) DeleteAccount(address common.Address, original *accounts.Account) error {
	cache.accountLock.Lock()
	defer cache.accountLock.Unlock()

	// Non-existent / deleted accounts are set to nil
	cache.accountCache[address] = nil
	return nil
}

func (cache *PlainStateCache) WriteAccountStorage(address common.Address, incarnation uint64, key *common.Hash, original, value *uint256.Int) error {
	cache.storageLock.Lock()
	defer cache.storageLock.Unlock()

	cache.storageCache[string(dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes()))] = value
	return nil
}

func (cache *PlainStateCache) CreateContract(codeHash common.Hash, code []byte) error {
	cache.codeLock.Lock()
	defer cache.codeLock.Unlock()

	cache.codeCache[codeHash] = code
	return nil
}

func (cache *PlainStateCache) ReadAccountData(address libcommon.Address) (*accounts.Account, error) {
	cache.accountLock.RLock()
	account, ok := cache.accountCache[address]
	cache.accountLock.RUnlock()
	if ok {
		return account, nil
	}

	// Cache miss, read from snapshot
	enc, err := cache.db.GetOne(kv.PlainState, address.Bytes())
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 {
		return nil, nil
	}
	acc := &accounts.Account{}
	if err = acc.DecodeForStorage(enc); err != nil {
		return nil, err
	}

	// Load to cache
	acc = cache.loadAccountFromSnapshot(address, acc)

	return acc, nil
}

func (cache *PlainStateCache) ReadAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash) ([]byte, error) {
	compositeKey := dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())

	cache.storageLock.RLock()
	storage, ok := cache.storageCache[string(compositeKey)]
	cache.storageLock.RUnlock()
	if ok {
		return storage.Bytes(), nil
	}

	// Cache miss, read from snapshot
	enc, err := cache.db.GetOne(kv.PlainState, compositeKey)
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 {
		return nil, nil
	}

	// Load to cache
	enc = cache.loadStorageFromSnapshot(string(compositeKey), enc)

	return enc, nil
}

func (cache *PlainStateCache) ReadAccountCode(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) ([]byte, error) {
	if bytes.Equal(codeHash.Bytes(), emptyCodeHash) {
		return nil, nil
	}

	cache.codeLock.RLock()
	code, ok := cache.codeCache[codeHash]
	cache.codeLock.RUnlock()
	if ok {
		return code, nil
	}

	// Cache miss, read from snapshot
	enc, err := cache.db.GetOne(kv.Code, codeHash.Bytes())
	if len(enc) == 0 {
		return nil, nil
	}

	// Load to cache
	code = cache.loadCodeFromSnapshot(codeHash, enc)

	return code, err
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

	// Cache miss, read from snapshot
	b, err := cache.db.GetOne(kv.IncarnationMap, address.Bytes())
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, nil
	}
	incarnation = binary.BigEndian.Uint64(b)

	// Load to cache
	incarnation = cache.loadAccountIncarnationFromSnapshot(address, incarnation)

	return incarnation, nil
}

func (cache *PlainStateCache) loadAccountFromSnapshot(address libcommon.Address, accountSnapshot *accounts.Account) *accounts.Account {
	cache.accountLock.Lock()
	defer cache.accountLock.Unlock()
	if account, ok := cache.accountCache[address]; ok {
		// Cache hit
		return account
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
		return value.Bytes()
	}

	// Load to cache
	valueSnapshot := uint256.NewInt(0).SetBytes(encSnapshot)
	cache.storageCache[key] = valueSnapshot
	return encSnapshot
}

func (cache *PlainStateCache) loadCodeFromSnapshot(key libcommon.Hash, codeSnapshot []byte) []byte {
	cache.codeLock.Lock()
	defer cache.codeLock.Unlock()
	if code, ok := cache.codeCache[key]; ok {
		// Cache hit
		return code
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
