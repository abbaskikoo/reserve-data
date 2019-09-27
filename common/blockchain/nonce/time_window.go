package nonce

import (
	"context"
	"math/big"
	"sync"
	"time"

	"github.com/KyberNetwork/reserve-data/common"
	ethereum "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type TimeWindow struct {
	address     ethereum.Address
	mu          sync.Mutex
	manualNonce *big.Int
	time        uint64 // last time nonce was requested
	window      uint64 // window time in millisecond
}

// be very very careful to set `window` param, if we set it to high value, it can lead to nonce lost making the whole pricing operation stuck
func NewTimeWindow(address ethereum.Address, window uint64) *TimeWindow {
	return &TimeWindow{
		address,
		sync.Mutex{},
		big.NewInt(0),
		0,
		window,
	}
}

func (tw *TimeWindow) GetAddress() ethereum.Address {
	return tw.address
}

func (tw *TimeWindow) getNonceFromNode(ethclient *ethclient.Client) (*big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	nonce, err := ethclient.PendingNonceAt(ctx, tw.GetAddress())
	return big.NewInt(int64(nonce)), err
}

func (tw *TimeWindow) MinedNonce(ethclient *ethclient.Client) (*big.Int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	nonce, err := ethclient.NonceAt(ctx, tw.GetAddress(), nil)
	return big.NewInt(int64(nonce)), err
}

func (tw *TimeWindow) GetNextNonce(ethclient *ethclient.Client) (*big.Int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	t := common.NowInMillis()
	if t-tw.time < tw.window {
		tw.time = t
		tw.manualNonce.Add(tw.manualNonce, ethereum.Big1)
		return tw.manualNonce, nil
	}
	nonce, err := tw.getNonceFromNode(ethclient)
	if err != nil {
		return big.NewInt(0), err
	}
	tw.time = t
	tw.manualNonce = nonce
	return tw.manualNonce, nil
}
