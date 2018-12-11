package avalanche

import (
	"math/bits"
)

const (
	AvalancheFinalizationScore uint16 = 128
)

type VoteRecord struct {
	votes      uint16
	confidence uint16
}

func NewVoteRecord() VoteRecord {
	return VoteRecord{
		votes:      0xaaaa,
		confidence: 0,
	}
}

func (vr VoteRecord) IsValid() bool {
	// fmt.Println(vr.confidence)
	return (vr.confidence & 0x01) == 1
}

func (vr VoteRecord) getConfidence() uint16 {
	return vr.confidence >> 1
}

func (vr VoteRecord) hasFinalized() bool {
	return vr.getConfidence() >= AvalancheFinalizationScore
}

func (vr *VoteRecord) regsiterVote(vote bool) bool {
	var voteInt uint16
	if vote {
		voteInt = 1
	}

	vr.votes = (vr.votes << 1) | voteInt

	bitCount := bits.OnesCount16(vr.votes & 0xff)
	yes := (bitCount > 6)
	no := (bitCount < 2)

	// Vote is inconclusive
	if !yes && !no {
		return false
	}

	// Vote is conclusive and agrees with our current state
	if vr.IsValid() == yes {
		vr.confidence += 2

		// Vote is conclusive but does not agree with our current state
	} else {
		vr.confidence = 0
		if yes {
			vr.confidence++
		}
	}

	return true
}
