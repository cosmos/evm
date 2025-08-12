package types

// Failed returns if the contract execution failed in vm errors
func (egr EstimateGasResponse) Failed() bool {
	return len(egr.VmError) > 0
}
