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
	blockHash := pindex

	// Query for random block should return false
	assertFalse(t, p.isAccepted(pindex))
	assertFalse(t, p.hasFinalized(pindex))

	// Add a new block. Check that it's added to the polls
	assertTrue(t, p.addBlockToReconcile(pindex))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, blockHash)

	// Newly added blocks are also considered rejected
	assertFalse(t, p.isAccepted(pindex))
	assertFalse(t, p.hasFinalized(pindex))

	// Vote for the block a few times
	r := Response{votes: []Vote{Vote{0, blockHash}}}

	for i := 0; i < 5; i++ {
		p.registerVotes(r)
		assertFalse(t, p.isAccepted(pindex))
		assertFalse(t, p.hasFinalized(pindex))
	}

	// Now it is accepted, but we can vote for it numerous times.
	for i := 0; i < AvalancheFinalizationScore; i++ {
		p.registerVotes(r)

		// Newly added blocks are also considered rejected.
		assertTrue(t, p.isAccepted(pindex))
		assertFalse(t, p.hasFinalized(pindex))
	}

	// As long as it is not finalized, we poll.
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, blockHash)

	// Now finalize the decision.
	r = Response{votes: []Vote{Vote{1, blockHash}}}
	p.registerVotes(r)
	assertBlockPollCount(t, p, 0)
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

	// As long as it is not finalized, we poll.
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, blockHash)

	// Now finalize the decision.
	p.registerVotes(r)
	assertBlockPollCount(t, p, 0)
	assertFalse(t, p.isAccepted(pindex))
	assertTrue(t, p.hasFinalized(pindex))

	// Adding the block twice does nothing.
	assertFalse(t, p.addBlockToReconcile(pindex))
	assertFalse(t, p.isAccepted(pindex))
	assertTrue(t, p.hasFinalized(pindex))
}

func TestMultiBlockRegister(t *testing.T) {
	p := NewProcessor()

	pindexA := blockIndex(65)
	blockHashA := pindexA

	pindexB := blockIndex(66)
	blockHashB := pindexB

	resp := Response{0, []Vote{Vote{0, blockHashA}, Vote{0, blockHashB}}}

	// Query for random block should return false
	assertFalse(t, p.isAccepted(pindexA))
	assertFalse(t, p.hasFinalized(pindexA))
	assertFalse(t, p.isAccepted(pindexB))
	assertFalse(t, p.hasFinalized(pindexB))

	// Start voting on block A.
	assertTrue(t, p.addBlockToReconcile(pindexA))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindexA)

	// Vote both blocks
	p.registerVotes(resp)

	// Start voting on block B after one vote
	assertTrue(t, p.addBlockToReconcile(pindexB))
	assertBlockPollCount(t, p, 2)
	assertPollExistsForBlock(t, p, pindexA)
	assertPollExistsForBlock(t, p, pindexB)

	// TODO: somehow the ABC code has blocks coming out correctly sorted by PoW desc
	// but I can't figure out how the blocks are getting sorted that way. The AvalancheProcessor
	// appears to assume its vote_records properties is natually sorted

	// Now it is rejected but we can vote for it numerous times
	for i := 0; i < AvalancheFinalizationScore+4; i++ {
		p.registerVotes(resp)
		assertFalse(t, p.hasFinalized(pindexA))
	}

	// Next vote will finalize block A
	p.registerVotes(resp)

	// We do not vote on A anymore
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindexB)

	// Next vote will finalize block B
	p.registerVotes(resp)

	// There is nothing left to vote on
	assertBlockPollCount(t, p, 0)
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

func assertBlockPollCount(t *testing.T, p *Processor, count int) {
	invs := p.getInvsForNextPoll()
	if len(invs) != count {
		t.Fatal("Should have exactly", count, "invs but have", len(invs))
	}
}

func assertPollExistsForBlock(t *testing.T, p *Processor, blockHash blockIndex) {
	found := false
	for _, inv := range p.getInvsForNextPoll() {
		if inv.targetHash == blockHash {
			found = true
		}
	}

	if !found {
		t.Fatal("No inv for hash", blockHash)
	}
}
