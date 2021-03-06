package common

const (
	MiningStatusMined       = "mined"
	MiningStatusFailed      = "failed"
	MiningStatusSubmitted   = "submitted"
	MiningStatusLost        = "lost"
	MiningStatusPending     = "pending"
	ExchangeStatusDone      = "done"
	ExchangeStatusPending   = "pending"
	ExchangeStatusFailed    = "failed"
	ExchangeStatusSubmitted = "submitted"
	ExchangeStatusCancelled = "cancelled"
	ExchangeStatusLost      = "lost"
	ActionDeposit           = "deposit"
	ActionTrade             = "trade"
	ActionWithdraw          = "withdraw"
	ActionSetRate           = "set_rates"
	ActionCancelSetRate     = "cancel_set_rates"
)

const (
	// maxGasPrice this value only use when it can't receive value from network contract
	HighBoundGasPrice float64 = 200.0
)
