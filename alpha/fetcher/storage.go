package fetcher

import (
	"github.com/KyberNetwork/reserve-data/common"
)

type Storage interface {
	StorePrice(map[common.TokenPairID]common.OnePrice) error
}