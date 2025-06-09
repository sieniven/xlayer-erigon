package state

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/holiman/uint256"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/dbutils"
	"github.com/ledgerwatch/erigon/core/types/accounts"
	"github.com/ledgerwatch/erigon/turbo/trie"
	zktypes "github.com/ledgerwatch/erigon/zk/types"
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

	incarnationLock  sync.RWMutex
	incarnationCache map[libcommon.Address]uint64
}

func NewPlainStateCache(db kv.Getter, snapshotHeight uint64) *PlainStateCache {
	return &PlainStateCache{
		snapshotHeight:   snapshotHeight,
		snapshotReader:   NewPlainStateReader(db),
		accountCache:     make(map[libcommon.Address]*accounts.Account, DefaultRealtimeCacheSize),
		storageCache:     make(map[string]*uint256.Int, DefaultRealtimeCacheSize),
		codeCache:        make(map[libcommon.Hash][]byte, DefaultRealtimeCacheSize),
		incarnationCache: make(map[libcommon.Address]uint64, DefaultRealtimeCacheSize),
	}
}

func (cache *PlainStateCache) ApplyChangeset(changeset *zktypes.Changeset) error {
	// Handle account data changes

	return nil
}

func (cache *PlainStateCache) ApplyChangesetToAccountData(changeset *zktypes.Changeset) (err error) {
	addressChanges := make(map[libcommon.Address]*accounts.Account)

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

	// Apply code changes

	// Apply incarnation changes

	// Apply storage changes

	return nil
}

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

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountData(address)
}

func (cache *PlainStateCache) ReadAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash) ([]byte, error) {
	compositeKey := dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())

	cache.storageLock.RLock()
	storage, ok := cache.storageCache[string(compositeKey)]
	cache.storageLock.RUnlock()
	if ok {
		return libcommon.Copy(storage.Bytes()), nil
	}

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountStorage(address, incarnation, key)
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

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountCode(address, incarnation, codeHash)
}

func (cache *PlainStateCache) ReadAccountCodeSize(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) (int, error) {
	code, err := cache.ReadAccountCode(address, incarnation, codeHash)
	return len(code), err
}

func (cache *PlainStateCache) ReadAccountIncarnation(address libcommon.Address) (uint64, error) {
	cache.incarnationLock.RLock()
	incarnation, ok := cache.incarnationCache[address]
	cache.incarnationLock.RUnlock()
	if ok {
		return incarnation, nil
	}

	// Cache miss, read from snapshot
	return cache.snapshotReader.ReadAccountIncarnation(address)
}

func (cache *PlainStateCache) getOrCreateAccount(address libcommon.Address, addressChanges map[libcommon.Address]*accounts.Account) (*accounts.Account, error) {
	account, ok := addressChanges[address]
	if !ok {
		var err error
		account, err = cache.ReadAccountData(address)
		if err != nil {
			return nil, err
		}

		if account == nil {
			// Non-existent account, create new account
			account, err = cache.createAccount(address)
			if err != nil {
				return nil, err
			}
		}
		addressChanges[address] = account
	}

	return account, nil
}

func (cache *PlainStateCache) createAccount(address libcommon.Address) (*accounts.Account, error) {
	prevIncarnation, err := cache.ReadAccountIncarnation(address)
	if err != nil {
		return nil, fmt.Errorf("createAccount failed: %v", err)
	}

	return &accounts.Account{
		Initialised:     true,
		Nonce:           0,
		Root:            libcommon.BytesToHash(trie.EmptyRoot[:]),
		CodeHash:        libcommon.BytesToHash(emptyCodeHash),
		PrevIncarnation: prevIncarnation,
		Incarnation:     prevIncarnation + 1,
	}, nil
}
