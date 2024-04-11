package adapters

import (
	"github.com/hcnet/go/exp/lightaurora/common"
	"github.com/hcnet/go/protocols/aurora/base"
	"github.com/hcnet/go/protocols/aurora/operations"
	"github.com/hcnet/go/support/errors"
	"github.com/hcnet/go/xdr"
)

func populateAllowTrustOperation(op *common.Operation, baseOp operations.Base) (operations.AllowTrust, error) {
	allowTrust := op.Get().Body.MustAllowTrustOp()

	var (
		assetType string
		code      string
		issuer    string
	)

	err := allowTrust.Asset.ToAsset(op.SourceAccount()).Extract(&assetType, &code, &issuer)
	if err != nil {
		return operations.AllowTrust{}, errors.Wrap(err, "xdr.Asset.Extract error")
	}

	flags := xdr.TrustLineFlags(allowTrust.Authorize)

	return operations.AllowTrust{
		Base: baseOp,
		Asset: base.Asset{
			Type:   assetType,
			Code:   code,
			Issuer: issuer,
		},

		Trustee:                        op.SourceAccount().Address(),
		Trustor:                        allowTrust.Trustor.Address(),
		Authorize:                      flags.IsAuthorized(),
		AuthorizeToMaintainLiabilities: flags.IsAuthorizedToMaintainLiabilitiesFlag(),
		// TODO:
		TrusteeMuxed:   "",
		TrusteeMuxedID: 0,
	}, nil
}
