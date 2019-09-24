package world

import (
	"github.com/KyberNetwork/reserve-data/common"
)

func (tw *TheWorld) getCoinbaseInfo(ep string) common.CoinbaseData {
	var (
		url    = ep
		result = common.CoinbaseData{}
	)

	err := tw.getPublic(url, &result)
	if err != nil {
		result.Error = err.Error()
		result.Valid = false
	} else {
		result.Valid = true
	}
	return result
}

func (tw *TheWorld) getGeminiBTCInfo(url string) common.GeminiETHBTCData {
	var (
		result = common.GeminiETHBTCData{}
	)

	err := tw.getPublic(url, &result)
	if err != nil {
		result.Error = err.Error()
		result.Valid = false
	} else {
		result.Valid = true
	}
	return result
}

func (tw *TheWorld) getGeminiUSDInfo(url string) common.GeminiETHUSDData {
	var (
		result = common.GeminiETHUSDData{}
	)

	err := tw.getPublic(url, &result)
	if err != nil {
		result.Error = err.Error()
		result.Valid = false
	} else {
		result.Valid = true
	}
	return result
}

func (tw *TheWorld) GetBTCInfo() (common.BTCData, error) {
	return common.BTCData{
		Coinbase: tw.getCoinbaseInfo(tw.endpoint.CoinbaseBTCEndpoint()),
		Gemini:   tw.getGeminiBTCInfo(tw.endpoint.GeminiBTCEndpoint()),
	}, nil
}
