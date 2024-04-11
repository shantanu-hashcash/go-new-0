package adapters

import (
	"github.com/hcnet/go/amount"
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/operations"
)

func populateCreateAccountOperation(op *common.Operation, baseOp operations.Base) (operations.CreateAccount, error) {
	createAccount := op.Get().Body.MustCreateAccountOp()

	return operations.CreateAccount{
		Base:            baseOp,
		StartingBalance: amount.String(createAccount.StartingBalance),
		Funder:          op.SourceAccount().Address(),
		Account:         createAccount.Destination.Address(),
	}, nil
}
