package common

import (
	"math/big"

	ethereum "github.com/ethereum/go-ethereum/common"
)

type TestExchange struct {
}

func (te TestExchange) ID() ExchangeID {
	return "binance"
}
func (te TestExchange) Address(token Token) (address ethereum.Address, supported bool) {
	return ethereum.Address{}, true
}
func (te TestExchange) Withdraw(token Token, amount *big.Int, address ethereum.Address, timepoint uint64) (string, error) {
	return "withdrawid", nil
}
func (te TestExchange) Trade(tradeType string, base Token, quote Token, rate float64, amount float64, timepoint uint64) (id string, done float64, remaining float64, finished bool, err error) {
	return "tradeid", 10, 5, false, nil
}
func (te TestExchange) CancelOrder(id, base, quote string) error {
	return nil
}
func (te TestExchange) MarshalText() (text []byte, err error) {
	return []byte("bittrex"), nil
}

func (te TestExchange) GetTradeHistory(fromTime, toTime uint64) (ExchangeTradeHistory, error) {
	return ExchangeTradeHistory{}, nil
}

func (te TestExchange) GetLiveExchangeInfos(tokenPairIDs []uint64) (ExchangeInfo, error) {
	return ExchangeInfo{}, nil
}
