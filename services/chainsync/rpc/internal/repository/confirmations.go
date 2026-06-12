package repository

// minBlockForConfirmations returns the maximum tx.block_number that can be considered "confirmable"
// at currentBlock with the given confirmations threshold.
//
// It guards against uint-underflow when currentBlock < confirmations.
// ok=false means there cannot be any confirmable txs yet.
func minBlockForConfirmations(currentBlock uint64, confirmations int32) (minBlock uint64, ok bool) {
	if confirmations <= 0 {
		return currentBlock, true
	}
	confirmU := uint64(confirmations)
	if currentBlock < confirmU {
		return 0, false
	}
	return currentBlock - confirmU, true
}

