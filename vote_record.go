package avalanche

type VoteRecord struct {
	votes      uint8
	consider   uint8
	confidence uint16
}

func NewVoteRecord(accepted bool) *VoteRecord {
	return &VoteRecord{
		votes:      0xaa,
		confidence: boolToUint16(accepted),
	}
}

func (vr VoteRecord) isAccepted() bool {
	return (vr.confidence & 0x01) == 1
}

func (vr VoteRecord) getConfidence() uint16 {
	return vr.confidence >> 1
}

func (vr VoteRecord) hasFinalized() bool {
	return vr.getConfidence() >= AvalancheFinalizationScore
}

// regsiterVote adds a new vote for an item and update confidence accordingly.
// Returns true if the acceptance or finalization state changed.
func (vr *VoteRecord) regsiterVote(err uint32) bool {
	vr.votes = (vr.votes << 1) | boolToUint8(err == 0)
	vr.consider = (vr.consider << 1) | boolToUint8(int32(err) >= 0)

	yes := countBits8(vr.votes&vr.consider&0xff) > 6
	if !yes {
		no := countBits8((-vr.votes-1)&vr.consider&0xff) > 6
		if !no {
			// The round is inconclusive
			return false
		}
	}

	// Vote is conclusive and agrees with our current state
	if vr.isAccepted() == yes {
		vr.confidence += 2
		return vr.getConfidence() == AvalancheFinalizationScore
	}

	// Vote is conclusive but does not agree with our current state
	vr.confidence = boolToUint16(yes)

	return true
}

func countBits8(i uint8) (count int) {
	for ; i > 0; i &= (i - 1) {
		count++
	}
	return count
}

func boolToUint8(b bool) (i uint8) {
	if b {
		i = 1
	}
	return i
}

func boolToUint16(b bool) (i uint16) {
	if b {
		i = 1
	}
	return i
}
