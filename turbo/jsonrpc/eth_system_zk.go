package jsonrpc

import (
	"context"
	"math/big"

	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/gasprice/gaspricecfg"
	"github.com/ledgerwatch/erigon/ethclient"
	"github.com/ledgerwatch/erigon/zk/apollo"
	"github.com/ledgerwatch/log/v3"
)

type RpcL1GasPriceTracker interface {
	GetLatestPrice() (*big.Int, error)
	GetLowestPrice() *big.Int
}

func (api *APIImpl) GasPrice(ctx context.Context) (*hexutil.Big, error) {
	// For X Layer with robust error handling and fallback mechanisms

	// Fallback gas price for emergency cases
	fallbackGasPrice := (*hexutil.Big)(big.NewInt(1000000000)) // 1 Gwei

	// Check if L2GasPricer is available
	if api.L2GasPricer == nil {
		log.Warn("[GasPrice API] L2GasPricer is nil, using fallback gas price")
		return fallbackGasPrice, nil
	}

	// Get current config with error handling
	var currentConfig gaspricecfg.Config
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("[GasPrice API] Panic when getting L2GasPricer config", "panic", r)
				// Use default config
				currentConfig = gaspricecfg.Config{Default: big.NewInt(1000000000)}
			}
		}()
		currentConfig = api.L2GasPricer.GetConfig()
	}()

	// Check if XLayer is configured
	if currentConfig.XLayer.Type != "" {
		// Safely get Apollo and ZK config for logging (optional)
		var zkConfig *ethconfig.Zk
		apolloEnabled := false

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Debug("[GasPrice API] Apollo config access failed, continuing without logging", "panic", r)
				}
			}()

			apolloConfig := apollo.UnsafeGetApolloConfig()
			if apolloConfig != nil {
				apolloConfig.Lock()
				zkConfig = apolloConfig.EthCfg.Zk
				apolloConfig.Unlock()
				apolloEnabled = apollo.IsApolloConfigL2GasPricerEnabled()
			}
		}()

		// Log configuration (only if successfully obtained)
		if zkConfig != nil {
			log.Debug("[GasPrice API] Complete gas price configuration",
				"xlayerType", currentConfig.XLayer.Type,
				"xlayerFactor", currentConfig.XLayer.Factor,
				"xlayerUpdatePeriod", currentConfig.XLayer.UpdatePeriod,
				"xlayerCongestionThreshold", currentConfig.XLayer.CongestionThreshold,
				"xlayerGasPriceUsdt", currentConfig.XLayer.GasPriceUsdt,
				"xlayerKafkaURL", currentConfig.XLayer.KafkaURL,
				"xlayerTopic", currentConfig.XLayer.Topic,
				"xlayerGroupID", currentConfig.XLayer.GroupID,
				"xlayerDefaultL1CoinPrice", currentConfig.XLayer.DefaultL1CoinPrice,
				"xlayerDefaultL2CoinPrice", currentConfig.XLayer.DefaultL2CoinPrice,
				"defaultPrice", currentConfig.Default,
				"maxPrice", currentConfig.MaxPrice,
				"blocks", currentConfig.Blocks,
				"percentile", currentConfig.Percentile,
				"ignorePrice", currentConfig.IgnorePrice,
				"apolloEnabled", apolloEnabled,
				"effectiveGasPriceForEthTransfer", zkConfig.EffectiveGasPriceForEthTransfer,
				"effectiveGasPriceForErc20Transfer", zkConfig.EffectiveGasPriceForErc20Transfer,
				"effectiveGasPriceForContractInvocation", zkConfig.EffectiveGasPriceForContractInvocation,
				"effectiveGasPriceForContractDeployment", zkConfig.EffectiveGasPriceForContractDeployment,
				"zkevmDefaultGasPrice", zkConfig.DefaultGasPrice,
				"zkevmMaxGasPrice", zkConfig.MaxGasPrice,
				"zkevmGasPriceFactor", zkConfig.GasPriceFactor)
		} else {
			log.Debug("[GasPrice API] Basic gas price configuration",
				"xlayerType", currentConfig.XLayer.Type,
				"xlayerFactor", currentConfig.XLayer.Factor,
				"defaultPrice", currentConfig.Default,
				"apolloEnabled", apolloEnabled)
		}

		// Log cache information (optional)
		if api.gasCache != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Debug("[GasPrice API] Gas cache access failed", "panic", r)
					}
				}()
				hash, cachedPrice := api.gasCache.GetLatest()
				rawGP := api.gasCache.GetLatestRawGP()
				log.Debug("[GasPrice API] Current cache state",
					"cachedGasPrice", cachedPrice,
					"rawGasPrice", rawGP,
					"latestHash", hash.Hex())
			}()
		}

		// Try to get X Layer gas price with fallback
		result, err := func() (*hexutil.Big, error) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("[GasPrice API] Panic in gasPriceXL, using fallback", "panic", r)
				}
			}()
			return api.gasPriceXL(ctx)
		}()

		if err == nil && result != nil {
			log.Debug("[GasPrice API] X Layer gas price returned",
				"gasPrice", result,
				"xlayerType", currentConfig.XLayer.Type)
			return result, nil
		} else {
			log.Warn("[GasPrice API] X Layer gas price failed, using default from config",
				"error", err,
				"xlayerType", currentConfig.XLayer.Type,
				"defaultPrice", currentConfig.Default)

			// Use default price from config as fallback
			if currentConfig.Default != nil && currentConfig.Default.Sign() > 0 {
				return (*hexutil.Big)(currentConfig.Default), nil
			}
		}
	}

	log.Debug("[Apollo Gas Price] Using standard ZK gas pricing")

	// Try standard ZK gas pricing with robust error handling
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		log.Warn("[Apollo Gas Price] Database access failed, using fallback", "error", err)
		return fallbackGasPrice, nil
	}
	defer tx.Rollback()

	cc, err := api.chainConfig(ctx, tx)
	if err != nil {
		log.Warn("[Apollo Gas Price] Chain config access failed, using fallback", "error", err)
		return fallbackGasPrice, nil
	}

	chainId := cc.ChainID
	if !api.isZkNonSequencer(chainId) {
		if api.gasTracker != nil {
			price, err := api.gasTracker.GetLatestPrice()
			if err == nil && price != nil && price.Sign() > 0 {
				log.Debug("[Apollo Gas Price] Standard ZK gas price returned",
					"gasPrice", price,
					"source", "gas_tracker",
					"chainId", chainId)
				return (*hexutil.Big)(price), nil
			} else {
				log.Warn("[Apollo Gas Price] Gas tracker failed, using fallback", "error", err)
			}
		} else {
			log.Warn("[Apollo Gas Price] Gas tracker is nil, using fallback")
		}
		return fallbackGasPrice, nil
	}

	if api.BaseAPI.gasless {
		var price hexutil.Big
		log.Debug("[Apollo Gas Price] Gasless mode enabled, returning zero gas price")
		return &price, nil
	}

	// Try L2 RPC fallback with error handling
	l2Price := fallbackGasPrice
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("[Apollo Gas Price] Panic in L2 RPC access", "panic", r)
			}
		}()

		l2RpcUrl := api.GetL2RpcUrl()
		if l2RpcUrl == "" {
			log.Warn("[Apollo Gas Price] L2 RPC URL is empty, using fallback")
			return
		}

		client, err := ethclient.DialContext(ctx, l2RpcUrl)
		if err != nil {
			log.Warn("[Apollo Gas Price] Failed to dial L2 RPC, using fallback", "url", l2RpcUrl, "error", err)
			return
		}
		defer client.Close()

		price, err := client.SuggestGasPrice(ctx)
		if err == nil && price != nil && price.Sign() > 0 {
			log.Debug("[Apollo Gas Price] L2 RPC suggested gas price returned",
				"gasPrice", price,
				"source", "l2_rpc",
				"chainId", chainId)
			l2Price = (*hexutil.Big)(price)
		} else {
			log.Warn("[Apollo Gas Price] L2 RPC gas price failed, using fallback", "error", err)
		}
	}()

	return l2Price, nil
}
