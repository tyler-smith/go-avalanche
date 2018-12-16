package avalanche

import (
	"testing"
	"time"
)

var (
	_negativeOne = -1
	negativeOne  = uint32(_negativeOne)
)

func TestVoteRecord(t *testing.T) {
	var vr *VoteRecord
	registerVoteAndCheck := func(vote uint32, state, finalized bool, confidence uint16) {
		vr.regsiterVote(vote)
		assertTrue(t, vr.isAccepted() == state)
		assertTrue(t, vr.hasFinalized() == finalized)
		assertTrue(t, vr.getConfidence() == confidence)
	}

	vr = NewVoteRecord(true)
	assertTrue(t, vr.isAccepted())
	assertFalse(t, vr.hasFinalized())
	assertTrue(t, vr.getConfidence() == 0)

	vr = NewVoteRecord(false)
	assertFalse(t, vr.isAccepted())
	assertFalse(t, vr.hasFinalized())
	assertTrue(t, vr.getConfidence() == 0)

	// We need to register 6 positive votes before we start counting.
	for i := uint16(0); i < 6; i++ {
		registerVoteAndCheck(0, false, false, 0)
	}

	// Next vote will flip state, and confidence will increase as long as we
	// vote yes.
	registerVoteAndCheck(0, true, false, 0)

	// A single neutral vote do not change anything.
	registerVoteAndCheck(negativeOne, true, false, 1)
	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(0, true, false, i)
	}

	// Two neutral votes will stall progress.
	registerVoteAndCheck(negativeOne, true, false, 7)
	registerVoteAndCheck(negativeOne, true, false, 7)
	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(0, true, false, 7)
	}

	// Now confidence will increase as long as we vote yes.
	for i := uint16(8); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(0, true, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(1, true, true, AvalancheFinalizationScore)

	// Now that we have two no votes, confidence stop increasing.
	for i := uint16(0); i < 5; i++ {
		registerVoteAndCheck(1, true, true,
			AvalancheFinalizationScore)
	}

	// Next vote will flip state, and confidence will increase as long as we
	// vote no.
	registerVoteAndCheck(1, false, false, 0)

	// A single neutral vote do not change anything.
	registerVoteAndCheck(negativeOne, false, false, 1)
	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(1, false, false, i)
	}

	// Two neutral votes will stall progress.
	registerVoteAndCheck(negativeOne, false, false, 7)
	registerVoteAndCheck(negativeOne, false, false, 7)
	for i := uint16(2); i < 8; i++ {
		registerVoteAndCheck(1, false, false, 7)
	}

	// Now confidence will increase as long as we vote no.
	for i := uint16(8); i < AvalancheFinalizationScore; i++ {
		registerVoteAndCheck(1, false, false, i)
	}

	// The next vote will finalize the decision.
	registerVoteAndCheck(0, false, true, AvalancheFinalizationScore)
}
func TestBlockRegister(t *testing.T) {
	var (
		connman = newConnman()
		p       = NewProcessor(connman)
		nodeID  = NodeID(0)

		updates   = []StatusUpdate{}
		blockHash = Hash(65)
		pindex    = blockForHash(blockHash)

		noVote      = Response{votes: []Vote{NewVote(1, blockHash)}}
		yesVote     = Response{votes: []Vote{NewVote(0, blockHash)}}
		neutralVote = Response{votes: []Vote{NewVote(negativeOne, blockHash)}}
	)
	connman.addNode(nodeID)

	assertUpdateCount := func(c int) {
		if len(updates) != c {
			t.Fatal("Expected", c, "updates")
		}
	}

	// Query for random block should return false
	assertFalse(t, p.IsAccepted(pindex))

	// Add a new block. Check that it's added to the polls
	assertTrue(t, p.AddTargetToReconcile(pindex))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindex)

	// Newly added blocks are also considered rejected
	assertTrue(t, p.IsAccepted(pindex))

	// Vote for the block a few times
	for i := 0; i < 6; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID, yesVote, &updates))
		assertTrue(t, p.IsAccepted(pindex))
		assertConfidence(t, p, pindex, 0)
		assertUpdateCount(0)
	}

	// A single neutral vote do not change anything.
	p.eventLoop()
	assertTrue(t, p.RegisterVotes(nodeID, neutralVote, &updates))
	assertTrue(t, p.IsAccepted(pindex))
	assertConfidence(t, p, pindex, 0)
	assertUpdateCount(0)

	for i := uint16(1); i < 7; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID, yesVote, &updates))
		assertTrue(t, p.IsAccepted(pindex))
		assertConfidence(t, p, pindex, i)
		assertUpdateCount(0)
	}

	// Two neutral votes will stall progress.
	for i := 0; i < 2; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID, neutralVote, &updates))
		assertTrue(t, p.IsAccepted(pindex))
		assertConfidence(t, p, pindex, 6)
		assertUpdateCount(0)
	}

	for i := 2; i < 8; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID, yesVote, &updates))
		assertTrue(t, p.IsAccepted(pindex))
		assertConfidence(t, p, pindex, 6)
		assertUpdateCount(0)
	}

	// We vote on it numerous times to finalize it
	for i := uint16(7); i < AvalancheFinalizationScore; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID, yesVote, &updates))
		assertTrue(t, p.IsAccepted(pindex))
		assertConfidence(t, p, pindex, i)
		assertUpdateCount(0)
	}

	// As long as it is not finalized, we poll.
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindex)

	// Now finalize the decision.
	p.eventLoop()
	assertTrue(t, p.RegisterVotes(nodeID, yesVote, &updates))
	assertUpdateCount(1)
	if updates[0].Hash != blockHash {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHash)
	}
	if updates[0].Status != StatusFinalized {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusFinalized)
	}
	updates = []StatusUpdate{}

	// Once the decision is finalized, there is no poll for it
	assertBlockPollCount(t, p, 0)

	// Now let's undo this and finalize rejection.
	assertTrue(t, p.AddTargetToReconcile(pindex))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindex)

	for i := 0; i < 6; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID, noVote, &updates))
		assertTrue(t, p.IsAccepted(pindex))
		assertUpdateCount(0)
	}

	// Now the state will flip.
	p.eventLoop()
	assertTrue(t, p.RegisterVotes(nodeID, noVote, &updates))
	assertFalse(t, p.IsAccepted(pindex))
	assertUpdateCount(1)
	if updates[0].Hash != blockHash {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHash)
	}
	if updates[0].Status != StatusRejected {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusAccepted)
	}
	updates = []StatusUpdate{}

	// Now it is rejected, but we can vote for it numerous times.
	for i := 1; i < AvalancheFinalizationScore; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID, noVote, &updates))
		assertFalse(t, p.IsAccepted(pindex))
		assertUpdateCount(0)
	}

	// As long as it is not finalized, we poll.
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindex)

	// Now finalize the decision.
	p.eventLoop()
	assertTrue(t, p.RegisterVotes(nodeID, yesVote, &updates))
	assertFalse(t, p.IsAccepted(pindex))
	assertUpdateCount(1)
	if updates[0].Hash != blockHash {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHash)
	}
	if updates[0].Status != StatusInvalid {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusInvalid)
	}
	updates = []StatusUpdate{}

	// Once the decision is finalized, there is no poll for it.
	assertBlockPollCount(t, p, 0)

	// Adding the block twice does nothing.
	assertTrue(t, p.AddTargetToReconcile(pindex))
	assertFalse(t, p.AddTargetToReconcile(pindex))
	assertTrue(t, p.IsAccepted(pindex))
}

func TestMultiBlockRegister(t *testing.T) {
	var (
		connman = newConnman()
		p       = NewProcessor(connman)
		nodeID0 = NodeID(0)
		nodeID1 = NodeID(1)

		updates = []StatusUpdate{}

		blockHashA = Hash(65)
		pindexA    = blockForHash(blockHashA)
		blockHashB = Hash(66)
		pindexB    = blockForHash(blockHashB)

		round          = p.GetRound()
		yesVoteForA    = Response{round, 0, []Vote{NewVote(0, blockHashA)}}
		yesVoteForB    = Response{round + 1, 0, []Vote{NewVote(0, blockHashB)}}
		yesVoteForBoth = Response{round + 1, 0, []Vote{NewVote(0, blockHashB), NewVote(0, blockHashA)}}
	)
	connman.addNode(nodeID0)
	connman.addNode(nodeID1)

	// TODO: Implement polling both nodes like in the ABC tests. Currently it
	// always uses node0. Need to fix the getSuitableNode logic in Processor

	// TODO: The ABC tests don't change these afaict
	// Figure out why this needs to be true
	pindexB.isInActiveChain = true

	assertUpdateCount := func(c int) {
		if len(updates) != c {
			t.Fatal("Expected", c, "updates")
		}
	}

	// Query for random block should return false
	assertFalse(t, p.IsAccepted(pindexA))
	assertFalse(t, p.IsAccepted(pindexB))

	// Start voting on block A.
	assertTrue(t, p.AddTargetToReconcile(pindexA))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindexA)
	p.eventLoop()
	assertTrue(t, p.RegisterVotes(nodeID0, yesVoteForA, &updates))
	assertUpdateCount(0)

	// Start voting on block B after one vote
	p.round++
	assertTrue(t, p.AddTargetToReconcile(pindexB))
	assertBlockPollCount(t, p, 2)

	// B should be first because it has more accumulated work
	invs := p.getInvsForNextPoll()
	if invs[0].targetHash != blockHashB {
		t.Fatal("Inv for block B should be first because it has more work")
	}
	if invs[1].targetHash != blockHashA {
		t.Fatal("Inv for block B should be first because it has more work")
	}

	// Let's vote for these blocks a few times
	for i := 0; i < 4; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID0, yesVoteForBoth, &updates))
		assertUpdateCount(0)
	}

	// Now it is accepted, but we can vote for it numerous times.
	for i := 0; i < AvalancheFinalizationScore; i++ {
		p.eventLoop()
		assertTrue(t, p.RegisterVotes(nodeID0, yesVoteForBoth, &updates))
		assertUpdateCount(0)
	}

	// TODO:
	// Running two iterration of the event loop so that vote gets triggerd on A
	// and B

	// Next vote will finalize block A
	p.eventLoop()
	assertTrue(t, p.RegisterVotes(nodeID0, yesVoteForBoth, &updates))
	assertUpdateCount(1)
	if updates[0].Hash != blockHashA {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHashA)
	}
	if updates[0].Status != StatusFinalized {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusAccepted)
	}
	updates = []StatusUpdate{}

	// We do not vote on A anymore
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindexB)

	// Next vote will finalize block B
	p.eventLoop()
	assertTrue(t, p.RegisterVotes(nodeID0, yesVoteForB, &updates))
	assertUpdateCount(1)
	if updates[0].Hash != blockHashB {
		t.Fatal("Update has incorrect hash. Got", updates[0].Hash, "but wanted:", blockHashB)
	}
	if updates[0].Status != StatusFinalized {
		t.Fatal("Update has incorrect status. Got", updates[0].Status, "but wanted:", StatusAccepted)
	}
	updates = []StatusUpdate{}

	// There is nothing left to vote on.
	assertBlockPollCount(t, p, 0)
}

func TestProcessorEventLoop(t *testing.T) {
	p := NewProcessor(newConnman())

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

func assertPollExistsForBlock(t *testing.T, p *Processor, b *Block) {
	found := false
	for _, inv := range p.getInvsForNextPoll() {
		if inv.targetHash == b.Hash() {
			found = true
		}
	}

	if !found {
		t.Fatal("No inv for hash", b.Hash())
	}
}

func assertConfidence(t *testing.T, p *Processor, b *Block, expectedC uint16) {
	if c := p.GetConfidence(b); c != expectedC {
		t.Fatal("Incorrect confidence. Got:", c, "Wanted:", expectedC)
	}
}

func TestPollAndResponse(t *testing.T) {
	var (
		connman = newConnman()
		p       = NewProcessor(connman)
		avanode = NodeID(0)

		updates = []StatusUpdate{}

		blockHash = Hash(65)
		pindex    = blockForHash(blockHash)
	)
	connman.addNode(avanode)

	assertUpdateCount := func(c int) {
		if len(updates) != c {
			t.Fatal("Expected", c, "updates")
		}
	}

	// Test that it returns the peer
	assertTrue(t, p.getSuitableNodeToQuery() == avanode)

	// Register a block and check it is added to the list of elements to poll
	assertTrue(t, p.AddTargetToReconcile(pindex))
	assertBlockPollCount(t, p, 1)
	assertPollExistsForBlock(t, p, pindex)

	// Trigger a poll on avanode
	round := p.GetRound()
	p.eventLoop()
	// TODO: We should put nodes on a request timer and make this assertion pass
	// assertTrue(t, p.getSuitableNodeToQuery() == NoNode)

	// Response to the request
	vote := Response{round, 0, []Vote{NewVote(0, blockHash)}}
	assertTrue(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	// Now that avanode fullfilled his request it is added back to the list of
	// queriable nodes
	assertTrue(t, p.getSuitableNodeToQuery() == avanode)

	// Sending response when not polled fails
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	// Trigger a poll on avanode
	round = p.GetRound()
	p.eventLoop()
	// TODO: We should put nodes on a request timer and make this assertion pass
	// assertTrue(t, p.getSuitableNodeToQuery() == NoNode)

	// Sending responses that do not match the request also fails.
	// 1. Too many results.
	p.eventLoop()
	vote = Response{round, 0, []Vote{NewVote(0, blockHash), NewVote(0, blockHash)}}
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	// 2. Not enough results.
	p.eventLoop()
	vote = Response{round, 0, []Vote{}}
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	// 3. Do not match the poll
	p.eventLoop()
	vote = Response{round, 0, []Vote{{}}}
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	// 4.Invalid round count. Request is not discarded
	p.eventLoop()
	vote = Response{round + 1, 0, []Vote{NewVote(0, blockHash)}}
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	vote = Response{round - 1, 0, []Vote{NewVote(0, blockHash)}}
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	// 5. Making request for invalid nodes do not work. Request is not discarded
	p.eventLoop()
	vote = Response{round, 0, []Vote{NewVote(0, blockHash)}}
	assertFalse(t, p.RegisterVotes(NodeID(1234), vote, &updates))
	assertUpdateCount(0)

	// Proper response gets processed and avanode is available again.
	vote = Response{round, 0, []Vote{NewVote(0, blockHash)}}
	assertTrue(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)

	// Out of order response are rejected.
	blockHashB := Hash(66)
	pindexB := blockForHash(blockHashB)
	assertTrue(t, p.AddTargetToReconcile(pindexB))

	p.eventLoop()
	vote = Response{round, 0, []Vote{NewVote(0, blockHash), NewVote(0, blockHashB)}}
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)
	assertTrue(t, p.getSuitableNodeToQuery() == avanode)

	// But they are accepted in order
	p.eventLoop()
	vote = Response{round, 0, []Vote{NewVote(0, blockHashB), NewVote(0, blockHash)}}
	assertTrue(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)
	assertTrue(t, p.getSuitableNodeToQuery() == avanode)

	// When a block is marked invalid, stop polling.
	pindexB.valid = false
	p.eventLoop()
	vote = Response{round, 0, []Vote{NewVote(0, blockHash)}}
	assertTrue(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)
	assertTrue(t, p.getSuitableNodeToQuery() == avanode)

	// Expire requests after some time.
	p.eventLoop()
	clock = stubClocker{time.Now().Add(1 * time.Minute)}
	assertFalse(t, p.RegisterVotes(avanode, vote, &updates))
	assertUpdateCount(0)
}
