package rpchelper

import (
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	types2 "github.com/ledgerwatch/erigon/core/types"
)

func (a *LogsFilterAggregator) insertRealtimeLogsFilter(sender Sub[*types2.Log]) (RealtimeLogsSubID, *LogsFilter) {
	filterId := RealtimeLogsSubID(generateSubscriptionID())
	filter := &LogsFilter{addrs: map[libcommon.Address]int{}, topics: map[libcommon.Hash]int{}, sender: sender}
	a.realtimeLogsFilters.Put(filterId, filter)
	return filterId, filter
}

func (a *LogsFilterAggregator) removeRealtimeLogsFilter(filterId RealtimeLogsSubID) bool {
	filter, ok := a.realtimeLogsFilters.Get(filterId)
	if !ok {
		return false
	}
	filter.Close()
	filter, ok = a.realtimeLogsFilters.Delete(filterId)
	if !ok {
		return false
	}
	return true
}

func (a *LogsFilterAggregator) distributeRealtimeLog(logs []*types2.Log) error {
	for _, log := range logs {
		a.realtimeLogsFilters.Range(func(k RealtimeLogsSubID, filter *LogsFilter) error {

			if filter.allAddrs == 0 {
				_, addrOk := filter.addrs[log.Address]
				if !addrOk {
					return nil
				}
			}
			if filter.allTopics == 0 {
				if !a.chooseTopics(filter, log.Topics) {
					return nil
				}
			}
			lg := &types2.Log{
				Address:     log.Address,
				Topics:      log.Topics,
				Data:        log.Data,
				BlockNumber: log.BlockNumber,
				TxHash:      log.TxHash,
				TxIndex:     uint(log.TxIndex),
				BlockHash:   log.BlockHash,
				Index:       uint(log.Index),
				Removed:     log.Removed,
			}
			filter.sender.Send(lg)
			return nil
		})
	}

	return nil
}
