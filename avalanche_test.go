package avalanche

import (
	"testing"
)

func TestVoteRecord(t *testing.T) {
	vr := NewVoteRecord()
	checkInitialVoteRecord(t, vr)

	registerVoteAndCheck(t, vr, true, false, false, 0) // 4/4
	registerVoteAndCheck(t, vr, true, false, false, 0) // 5/3
	registerVoteAndCheck(t, vr, true, false, false, 0) // 5/3
	registerVoteAndCheck(t, vr, true, false, false, 0) // 6/2
	registerVoteAndCheck(t, vr, true, false, false, 0) // 6/2

	// Next vote will flip state, and confidence will increase as long as we
	// vote yes.
	for i := uint16(0); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(t, vr, true, true, false, i)
	}

	// The next vote will finalize the decision
	registerVoteAndCheck(t, vr, false, true, true, AvalancheFinalizationScore)

	// Now that we have two no votes confidence stops increasing
	for i := 0; i < 5; i++ {
		registerVoteAndCheck(t, vr, false, true, true, AvalancheFinalizationScore)
	}

	// Next vote will flip state, and confidence will increase as long as we
	// vote no.
	for i := uint16(0); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(t, vr, false, false, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(t, vr, true, false, true, AvalancheFinalizationScore)
}

func checkInitialVoteRecord(t *testing.T, vr *VoteRecord) {
	if vr.isValid() {
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

	if vr.isValid() != state {
		t.Fatal("Expected IsValid to be", state, "but it was not")
	}

	if vr.getConfidence() != confidence {
		t.Fatal("Expected getConfidence to be", confidence, "but it was", vr.getConfidence())
	}

	if vr.hasFinalized() != finalized {
		t.Fatal("Expected hasFinalized to be", finalized, "but it was not")
	}
}
func TestBlockRegister(t *testing.T) {
	p := NewProcessor()

	pindex := blockIndex(42)
	hash := pindex

	// Query for random block should return false
	assertFalse(t, p.isAccepted(pindex))
	assertFalse(t, p.hasFinalized(pindex))

	// Newly added blocks are also considered rejected
	assertTrue(t, p.addBlockToReconcile(pindex))
	assertFalse(t, p.isAccepted(pindex))
	assertFalse(t, p.hasFinalized(pindex))

	// Vote for the block a few times
	r := Response{votes: []Vote{Vote{0, hash}}}

	for i := 0; i < 5; i++ {
		p.registerVotes(r)
		assertFalse(t, p.isAccepted(pindex))
		assertFalse(t, p.hasFinalized(pindex))
	}

	// Now it is accepted, but we can vote for it numerous times.
	for i := 0; i < AvalancheFinalizationScore; i++ {
		p.registerVotes(r)
		assertTrue(t, p.isAccepted(pindex))
		assertFalse(t, p.hasFinalized(pindex))
	}

	// Now finalize the decision.
	r = Response{votes: []Vote{Vote{1, hash}}}
	p.registerVotes(r)
	assertTrue(t, p.isAccepted(pindex))
	assertTrue(t, p.hasFinalized(pindex))

	// Now let's undo this and finalize rejection.
	for i := 0; i < 5; i++ {
		p.registerVotes(r)
		assertTrue(t, p.isAccepted(pindex))
		assertTrue(t, p.hasFinalized(pindex))
	}

	// Now it is rejected, but we can vote for it numerous times.
	for i := 0; i < AvalancheFinalizationScore; i++ {
		p.registerVotes(r)
		assertFalse(t, p.isAccepted(pindex))
		assertFalse(t, p.hasFinalized(pindex))
	}

	// Now finalize the decision.
	p.registerVotes(r)
	assertFalse(t, p.isAccepted(pindex))
	assertTrue(t, p.hasFinalized(pindex))

	// Adding the block twice does nothing.
	assertFalse(t, p.addBlockToReconcile(pindex))
	assertFalse(t, p.isAccepted(pindex))
	assertTrue(t, p.hasFinalized(pindex))
}

func TestProcessorEventLoop(t *testing.T) {
	p := NewProcessor()

	// Start loop
	assertTrue(t, p.start())

	// Can't start it twice
	assertFalse(t, p.start())

	// Stop loop
	assertTrue(t, p.stop())

	// Can't stop twice
	assertFalse(t, p.stop())

	// You can restart it and stop it again
	assertTrue(t, p.start())
	assertTrue(t, p.stop())
}

func assertTrue(t *testing.T, actual bool) {
	if !actual {
		t.Fatal("Expected true; got false")
	}
}

func assertFalse(t *testing.T, actual bool) {
	if actual {
		t.Fatal("Expected false; got true")
	}
}
