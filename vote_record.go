package avalanche

import "fmt"

type VoteRecord struct {
	votes      uint8
	consider   uint8
	confidence uint16
}

func NewVoteRecord(accepted bool) *VoteRecord {
	// return &VoteRecord{}
	return &VoteRecord{
		votes: 0xaa,
		// confidence: boolToUint16(accepted),
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

	fmt.Println(fmt.Sprintf("vote: %d, yes: %d", err, countBits8(vr.votes&vr.consider&0xff)))
	// fmt.Println("vote:", err)
	// fmt.Println("votes:", vr.votes)
	// fmt.Println("consider:", vr.consider)
	// fmt.Println("confidence:", vr.getConfidence())
	// fmt.Println("yes:", countBits8(vr.votes&vr.consider&0xff))
	// fmt.Println("no:", countBits8((-vr.votes-1)&vr.consider&0xff))

	if !yes {
		// fmt.Println("!yes", countBits8(vr.votes&vr.consider&0xff))
		// (-x-1) is equal to C's ~x
		no := countBits8((-vr.votes-1)&vr.consider&0xff) > 6
		if !no {
			fmt.Println("Inconclusive.")
			// fmt.Println("!no", countBits8((-vr.votes-1)&vr.consider&0xff))
			// The round is inconclusive
			return false
		}
	}

	// Vote is conclusive and agrees with our current state
	if vr.isAccepted() == yes {
		fmt.Println("Accepted.")
		vr.confidence += 2
		return vr.getConfidence() == AvalancheFinalizationScore
	}

	// Vote is conclusive but does not agree with our current state
	vr.confidence = boolToUint16(yes)

	fmt.Println("Vote flipped to", vr.isAccepted())

	return true
}

func countBits8(i uint8) (count int) {
	for ; i > 0; i &= (i - 1) {
		count++
	}
	return count
}

func countBits16(i uint16) (count int) {
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
