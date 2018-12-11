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
	if vr.isAccepted() {
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

	if vr.isAccepted() != state {
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
	updates := []StatusUpdate{}

	blockHash := Hash(65)
	pindex := blockHash

	assertUpdateCount := func(c int) {
		if len(updates) != c {
			t.Fatal("Expected", c, "updates")
		}
	}

	// Query for random block should return false
	assertFalse(t, p.isAccepted(pindex))

	// Add a new block. Check that it's added to the polls
	assertTrue(t, p.addBlockToReconcile(pindex))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, blockHash)

	// Newly added blocks are also considered rejected
	assertFalse(t, p.isAccepted(pindex))

	// Vote for the block a few times
	r := Response{votes: []Vote{Vote{0, blockHash}}}

	for i := 0; i < 5; i++ {
		p.registerVotes(r, &updates)
		assertFalse(t, p.isAccepted(pindex))
		assertUpdateCount(0)
	}

	// Now the state will flip.
	p.registerVotes(r, &updates)
	assertTrue(t, p.isAccepted(pindex))
	assertUpdateCount(1)
	if updates[0].Hash != blockHash {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHash)
	}
	if updates[0].Status != StatusAccepted {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusAccepted)
	}
	updates = []StatusUpdate{}

	// Now it is accepted, but we can vote for it numerous times.
	for i := 1; i < AvalancheFinalizationScore; i++ {
		p.registerVotes(r, &updates)
		assertTrue(t, p.isAccepted(pindex))
		assertUpdateCount(0)
	}

	// As long as it is not finalized, we poll.
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, blockHash)

	// Now finalize the decision.
	r = Response{votes: []Vote{Vote{1, blockHash}}}
	p.registerVotes(r, &updates)
	assertBlockPollCount(t, p, 0)
	assertUpdateCount(1)
	if updates[0].Hash != blockHash {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHash)
	}
	if updates[0].Status != StatusFinalized {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusFinalized)
	}
	updates = []StatusUpdate{}

	// Now let's undo this and finalize rejection.
	assertTrue(t, p.addBlockToReconcile(pindex))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, blockHash)

	// Only 3 here as we don't need to flip state
	for i := 0; i < 3; i++ {
		p.registerVotes(r, &updates)
		assertFalse(t, p.isAccepted(pindex))
		assertUpdateCount(0)
	}

	// Now it is rejected, but we can vote for it numerous times.
	for i := 0; i < AvalancheFinalizationScore; i++ {
		p.registerVotes(r, &updates)
		assertFalse(t, p.isAccepted(pindex))
		assertUpdateCount(0)
	}

	// As long as it is not finalized, we poll.
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, blockHash)

	// Now finalize the decision.
	p.registerVotes(r, &updates)
	assertBlockPollCount(t, p, 0)
	assertFalse(t, p.isAccepted(pindex))
	assertUpdateCount(1)
	if updates[0].Hash != blockHash {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHash)
	}
	if updates[0].Status != StatusInvalid {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusInvalid)
	}

	// Adding the block twice does nothing.
	assertTrue(t, p.addBlockToReconcile(pindex))
	assertFalse(t, p.isAccepted(pindex))
	assertFalse(t, p.isAccepted(pindex))
}

func TestMultiBlockRegister(t *testing.T) {
	p := NewProcessor()
	updates := []StatusUpdate{}

	pindexA := Hash(65)
	blockHashA := pindexA

	pindexB := Hash(66)
	blockHashB := pindexB

	resp := Response{0, []Vote{Vote{0, blockHashA}, Vote{0, blockHashB}}}

	assertUpdateCount := func(c int) {
		if len(updates) != c {
			t.Fatal("Expected", c, "updates")
		}
	}

	// Query for random block should return false
	assertFalse(t, p.isAccepted(pindexA))
	assertFalse(t, p.isAccepted(pindexB))

	// Start voting on block A.
	assertTrue(t, p.addBlockToReconcile(pindexA))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindexA)

	// Vote on block A
	p.registerVotes(resp, &updates)
	assertUpdateCount(0)

	// Start voting on block B after one vote
	assertTrue(t, p.addBlockToReconcile(pindexB))
	assertBlockPollCount(t, p, 2)

	// B should be first because it has more accumulated work
	invs := p.getInvsForNextPoll()
	if invs[0].targetHash != blockHashB {
		t.Fatal("Inv for block B should be first because it has more work")
	}
	if invs[1].targetHash != blockHashA {
		t.Fatal("Inv for block B should be first because it has more work")
	}

	// Let's vote for this block a few times
	for i := 0; i < 4; i++ {
		p.registerVotes(resp, &updates)
		assertUpdateCount(0)
	}

	// Now the state will flip for A
	p.registerVotes(resp, &updates)
	assertUpdateCount(1)
	if updates[0].Hash != blockHashA {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHashA)
	}
	if updates[0].Status != StatusAccepted {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusAccepted)
	}
	updates = []StatusUpdate{}

	// And then for B
	p.registerVotes(resp, &updates)
	assertUpdateCount(1)
	if updates[0].Hash != blockHashB {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHashB)
	}
	if updates[0].Status != StatusAccepted {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusAccepted)
	}
	updates = []StatusUpdate{}

	// Now it is rejected but we can vote for it numerous times
	for i := 2; i < AvalancheFinalizationScore; i++ {
		p.registerVotes(resp, &updates)
		assertUpdateCount(0)
	}

	// Next vote will finalize block A
	p.registerVotes(resp, &updates)
	assertUpdateCount(1)
	if updates[0].Hash != blockHashA {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHashA)
	}
	if updates[0].Status != StatusFinalized {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusFinalized)
	}
	updates = []StatusUpdate{}

	// We do not vote on A anymore
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindexB)

	// Next vote will finalize block B
	p.registerVotes(resp, &updates)
	assertUpdateCount(1)
	if updates[0].Hash != blockHashB {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHashB)
	}
	if updates[0].Status != StatusFinalized {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusFinalized)
	}
	updates = []StatusUpdate{}

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
		panic("")
		t.Fatal("Expected true; got false")
	}
}

func assertFalse(t *testing.T, actual bool) {
	if actual {
		panic("")
		t.Fatal("Expected false; got true")
	}
}

func assertBlockPollCount(t *testing.T, p *Processor, count int) {
	invs := p.getInvsForNextPoll()
	if len(invs) != count {
		t.Fatal("Should have exactly", count, "invs but have", len(invs))
	}
}

func assertPollExistsForBlock(t *testing.T, p *Processor, blockHash Hash) {
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
