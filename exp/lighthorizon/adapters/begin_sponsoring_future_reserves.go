package adapters

import (
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/operations"
)

func populateBeginSponsoringFutureReservesOperation(op *common.Operation, baseOp operations.Base) (operations.BeginSponsoringFutureReserves, error) {
	beginSponsoringFutureReserves := op.Get().Body.MustBeginSponsoringFutureReservesOp()

	return operations.BeginSponsoringFutureReserves{
		Base:        baseOp,
		SponsoredID: beginSponsoringFutureReserves.SponsoredId.Address(),
	}, nil
}
