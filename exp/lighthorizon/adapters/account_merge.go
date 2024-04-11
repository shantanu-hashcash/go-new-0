package adapters

import (
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/operations"
)

func populateAccountMergeOperation(op *common.Operation, baseOp operations.Base) (operations.AccountMerge, error) {
	destination := op.Get().Body.MustDestination()

	return operations.AccountMerge{
		Base:    baseOp,
		Account: op.SourceAccount().Address(),
		Into:    destination.Address(),
		// TODO:
		AccountMuxed:   "",
		AccountMuxedID: 0,
		IntoMuxed:      "",
		IntoMuxedID:    0,
	}, nil
}
