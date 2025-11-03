package ics02

import (
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
)

func (h *Height) FromProofHeight(ch clienttypes.Height) {
	h.RevisionNumber = ch.RevisionNumber
	h.RevisionHeight = ch.RevisionHeight
}

func (h Height) ToProofHeight() clienttypes.Height {
	return clienttypes.Height{
		RevisionNumber: h.RevisionNumber,
		RevisionHeight: h.RevisionHeight,
	}
}
