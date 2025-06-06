package state

import (
	"bytes"
	"encoding/binary"

	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/dbutils"
	"github.com/ledgerwatch/erigon/core/types/accounts"
)

const (
	DefaultRealtimeCacheSize = 1_000_000
)

type PlainStateCacheReader struct {
	db             kv.Getter
	accountChanges map[libcommon.Address][]byte
	storageChanges map[string][]byte
}

func NewPlainStateCacheReader(db kv.Getter) *PlainStateCacheReader {
	return &PlainStateCacheReader{
		db:             db,
		accountChanges: make(map[libcommon.Address][]byte, DefaultRealtimeCacheSize),
		storageChanges: make(map[string][]byte, DefaultRealtimeCacheSize),
	}
}

func (cache *PlainStateCacheReader) ReadAccountData(address libcommon.Address) (*accounts.Account, error) {
	if account, ok := cache.accountChanges[address]; ok {
		var acc accounts.Account
		if err := acc.DecodeForStorage(account); err != nil {
			return nil, err
		}
		return &acc, nil
	}

	// Cache miss, read from snapshot and load to cache
	enc, err := cache.db.GetOne(kv.PlainState, address.Bytes())
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 {
		return nil, nil
	}
	var a accounts.Account
	if err = a.DecodeForStorage(enc); err != nil {
		return nil, err
	}
	cache.accountChanges[address] = enc

	return &a, nil

}

func (cache *PlainStateCacheReader) ReadAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash) ([]byte, error) {
	compositeKey := dbutils.PlainGenerateCompositeStorageKey(address.Bytes(), incarnation, key.Bytes())
	if enc, ok := cache.storageChanges[string(compositeKey)]; ok {
		return enc, nil
	}

	// Cache miss, read from snapshot and load to cache
	enc, err := cache.db.GetOne(kv.PlainState, compositeKey)
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 {
		return nil, nil
	}
	return enc, nil
}

func (cache *PlainStateCacheReader) ReadAccountCode(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) ([]byte, error) {
	if bytes.Equal(codeHash.Bytes(), emptyCodeHash) {
		return nil, nil
	}

	if account, ok := cache.accountChanges[address]; ok {
		var acc accounts.Account
		if err := acc.DecodeForStorage(account); err != nil {
			return nil, err
		}
		return acc.CodeHash.Bytes(), nil
	}

	// Cache miss, read from snapshot and load to cache
	code, err := cache.db.GetOne(kv.Code, codeHash.Bytes())
	if len(code) == 0 {
		return nil, nil
	}
	return code, err
}

func (cache *PlainStateCacheReader) ReadAccountCodeSize(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) (int, error) {
	code, err := cache.ReadAccountCode(address, incarnation, codeHash)
	return len(code), err
}

func (cache *PlainStateCacheReader) ReadAccountIncarnation(address libcommon.Address) (uint64, error) {
	if account, ok := cache.accountChanges[address]; ok {
		var acc accounts.Account
		if err := acc.DecodeForStorage(account); err != nil {
			return 0, err
		}
		return acc.Incarnation, nil
	}

	// Cache miss, read from snapshot and load to cache
	b, err := cache.db.GetOne(kv.IncarnationMap, address.Bytes())
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, nil
	}
	return binary.BigEndian.Uint64(b), nil
}
