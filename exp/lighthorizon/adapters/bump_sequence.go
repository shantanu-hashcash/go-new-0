package adapters

import (
	"strconv"

	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
)

func populateBumpSequenceOperation(op *common.Operation, baseOp operations.Base) (operations.BumpSequence, error) {
	bumpSequence := op.Get().Body.MustBumpSequenceOp()

	return operations.BumpSequence{
		Base:   baseOp,
		BumpTo: strconv.FormatInt(int64(bumpSequence.BumpTo), 10),
	}, nil
}
