package adapters

import (
	"github.com/hcnet/go/amount"
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/base"
	"github.com/hcnet/go/protocols/aurora/operations"
	"github.com/hcnet/go/support/errors"
)

func populateClawbackOperation(op *common.Operation, baseOp operations.Base) (operations.Clawback, error) {
	clawback := op.Get().Body.MustClawbackOp()

	var (
		assetType string
		code      string
		issuer    string
	)
	err := clawback.Asset.Extract(&assetType, &code, &issuer)
	if err != nil {
		return operations.Clawback{}, errors.Wrap(err, "xdr.Asset.Extract error")
	}

	return operations.Clawback{
		Base: baseOp,
		Asset: base.Asset{
			Type:   assetType,
			Code:   code,
			Issuer: issuer,
		},
		Amount: amount.String(clawback.Amount),
		From:   clawback.From.Address(),
		// TODO:
		FromMuxed:   "",
		FromMuxedID: 0,
	}, nil
}
