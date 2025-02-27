package balance

import (
	"github.com/baely/balance/pkg/model"
)

type TransactionEvent struct {
	Account     model.AccountResource
	Transaction model.TransactionResource
}

type TransactionEventHandler interface {
	HandleEvent(transactionEvent TransactionEvent)
}
