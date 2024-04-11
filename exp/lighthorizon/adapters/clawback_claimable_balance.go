package adapters

import (
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/operations"
	"github.com/hcnet/go/support/errors"
	"github.com/hcnet/go/xdr"
)

func populateClawbackClaimableBalanceOperation(op *common.Operation, baseOp operations.Base) (operations.ClawbackClaimableBalance, error) {
	clawbackClaimableBalance := op.Get().Body.MustClawbackClaimableBalanceOp()

	balanceID, err := xdr.MarshalHex(clawbackClaimableBalance.BalanceId)
	if err != nil {
		return operations.ClawbackClaimableBalance{}, errors.Wrap(err, "invalid balanceId")
	}

	return operations.ClawbackClaimableBalance{
		Base:      baseOp,
		BalanceID: balanceID,
	}, nil
}
