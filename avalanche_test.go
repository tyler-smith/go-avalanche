package avalanche

import (
	"testing"
)

func TestVoteRecord(t *testing.T) {
	vr := NewVoteRecord()
	checkInitialVoteRecord(t, vr)

	registerVoteAndCheck(t, &vr, true, false, false, 0) // 4/4
	registerVoteAndCheck(t, &vr, true, false, false, 0) // 5/3
	registerVoteAndCheck(t, &vr, true, false, false, 0) // 5/3
	registerVoteAndCheck(t, &vr, true, false, false, 0) // 6/2
	registerVoteAndCheck(t, &vr, true, false, false, 0) // 6/2

	// Next vote will flip state, and confidence will increase as long as we
	// vote yes.
	for i := uint16(0); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(t, &vr, true, true, false, i)
	}

	// The next vote will finalize the decision
	registerVoteAndCheck(t, &vr, false, true, true, AvalancheFinalizationScore)

	// Now that we have two no votes confidence stops increasing
	for i := 0; i < 5; i++ {
		registerVoteAndCheck(t, &vr, false, true, true, AvalancheFinalizationScore)
	}

	// Next vote will flip state, and confidence will increase as long as we
	// vote no.
	for i := uint16(0); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(t, &vr, false, false, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(t, &vr, true, false, true, AvalancheFinalizationScore)
}

func checkInitialVoteRecord(t *testing.T, vr VoteRecord) {
	if vr.IsValid() {
		t.Fatal("Expected isValid to be false but it was true")
	}

	if vr.hasFinalized() {
		t.Fatal("Expected hasFinalized to be false but it was true")
	}

	if vr.getConfidence() != 0 {
		t.Fatal("Expected getConfidence to be 0 but it was", vr.getConfidence())
	}
}

func registerVoteAndCheck(t *testing.T, vr *VoteRecord, vote, state, finalized bool, confidence uint16) {
	vr.regsiterVote(vote)

	if vr.IsValid() != state {
		t.Fatal("Expected IsValid to be", state, "but it was not")
	}

	if vr.getConfidence() != confidence {
		t.Fatal("Expected getConfidence to be", confidence, "but it was", vr.getConfidence())
	}

	if vr.hasFinalized() != finalized {
		t.Fatal("Expected hasFinalized to be", finalized, "but it was not")
	}
}
