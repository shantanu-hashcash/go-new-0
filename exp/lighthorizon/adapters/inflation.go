package adapters

import (
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/operations"
)

func populateInflationOperation(op *common.Operation, baseOp operations.Base) (operations.Inflation, error) {
	return operations.Inflation{
		Base: baseOp,
	}, nil
}
