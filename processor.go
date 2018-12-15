package avalanche

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Processor drives the Avalanche process by sending queries and handling
// responses.
type Processor struct {
	voteRecords map[Hash]*VoteRecord
	round       int64
	queries     map[string]RequestRecord
	nodeIDs     map[NodeID]struct{}

	connman *connman

	runMu     sync.Mutex
	isRunning bool
	quitCh    chan (struct{})
	doneCh    chan (struct{})
}

// NewProcessor creates a new *Processor
func NewProcessor(connman *connman) *Processor {
	return &Processor{
		voteRecords: map[Hash]*VoteRecord{},
		queries:     map[string]RequestRecord{},
		nodeIDs:     map[NodeID]struct{}{},

		connman: connman,
	}
}

// GetRound returns the current round for the *Processor
func (p *Processor) GetRound() int64 {
	return p.round
}

// AddBlockToReconcile begins the voting process for a given target
func (p *Processor) AddBlockToReconcile(t Target) bool {
	if !p.isWorthyPolling(t) {
		return false
	}

	_, ok := p.voteRecords[t.Hash()]
	if ok {
		return false
	}

	p.voteRecords[t.Hash()] = NewVoteRecord(t.IsAccepted())
	return true
}

// RegisterVotes processes responses to queries
func (p *Processor) RegisterVotes(id NodeID, resp Response, updates *[]StatusUpdate) bool {
	key := queryKey(resp.GetRound(), id)

	r, ok := p.queries[key]
	if !ok {
		return false
	}

	// Always delete the key if it's present
	delete(p.queries, key)

	if r.IsExpired() {
		return false
	}

	invs := r.GetInvs()
	votes := resp.GetVotes()

	if len(votes) != len(invs) {
		return false
	}

	for i, v := range votes {
		if invs[i].targetHash != v.GetHash() {
			return false
		}
	}

	for _, v := range votes {
		vr, ok := p.voteRecords[v.GetHash()]
		if !ok {
			// We are not voting on this anymore
			continue
		}

		if !p.isWorthyPolling(blockForHash(v.GetHash())) {
			continue
		}

		if !vr.regsiterVote(v.GetError()) {
			// This vote did not provide any extra information
			continue
		}

		// Add appropriate status
		var status Status
		finalized := vr.hasFinalized()
		accepted := vr.isAccepted()
		switch {
		case !finalized && accepted:
			status = StatusAccepted
		case !finalized && !accepted:
			status = StatusRejected
		case finalized && accepted:
			status = StatusFinalized
		case finalized && !accepted:
			status = StatusInvalid
		}

		*updates = append(*updates, StatusUpdate{v.GetHash(), status})

		// When we finalize we want to remove our vote record
		if finalized {
			delete(p.voteRecords, v.GetHash())
		}
	}

	p.nodeIDs[id] = struct{}{}

	return true
}

// IsAccepted returns whether or not the Traget has been accepted by consensus
func (p *Processor) IsAccepted(t Target) bool {
	if vr, ok := p.voteRecords[t.Hash()]; ok {
		return vr.isAccepted()
	}
	return false
}

// GetConfidence returns the confidence we have in the Target's acceptance
func (p *Processor) GetConfidence(t Target) uint16 {
	vr, ok := p.voteRecords[t.Hash()]
	if !ok {
		panic("VoteRecord not found")
	}

	return vr.getConfidence()
}

// getInvsForNextPoll returns Invs for outstanding items that need to be
// resolved by further queries
func (p *Processor) getInvsForNextPoll() []Inv {
	invs := make([]Inv, 0, len(p.voteRecords))

	for idx, r := range p.voteRecords {
		if r.hasFinalized() {
			// If this has finalized we can just skip.
			continue
		}

		// Obviously do not poll if the target is not worth polling
		if !p.isWorthyPolling(blockForHash(idx)) {
			continue
		}

		// We don't have a decision, we need more votes.
		invs = append(invs, Inv{"block", idx})
	}

	sortBlockInvsByWork(invs)

	if len(invs) >= AvalancheMaxElementPoll {
		invs = invs[:AvalancheMaxElementPoll]
	}

	return invs
}

// getSuitableNodeToQuery returns the best node to send the next query to
func (p *Processor) getSuitableNodeToQuery() NodeID {
	nodeIDs := p.connman.nodesIDs()

	sort.Sort(nodesInRequestOrder(nodeIDs))

	if len(nodeIDs) == 0 {
		return NoNode
	}
	return nodeIDs[0]
}

// isWorthyPolling determines whether or it's even worth polling about a Target
func (p *Processor) isWorthyPolling(t Target) bool {
	return t.IsValid()
}

// start begins the poll/response cycle
func (p *Processor) start() bool {
	p.runMu.Lock()
	defer p.runMu.Unlock()

	if p.isRunning {
		return false
	}

	p.isRunning = true
	p.quitCh = make(chan (struct{}))
	p.doneCh = make(chan (struct{}))

	go func() {
		t := time.NewTicker(AvalancheTimeStep)
		for {
			select {
			case <-p.quitCh:
				close(p.doneCh)
				return
			case <-t.C:
				p.eventLoop()
			}
		}
	}()

	return true
}

// stop ends the poll/response cycle
func (p *Processor) stop() bool {
	p.runMu.Lock()
	defer p.runMu.Unlock()

	if !p.isRunning {
		return false
	}

	close(p.quitCh)
	<-p.doneCh

	p.isRunning = false
	return true
}

// eventLoop performs a tick of processing
func (p *Processor) eventLoop() {
	invs := p.getInvsForNextPoll()
	if len(invs) == 0 {
		return
	}

	nodeID := p.getSuitableNodeToQuery()
	p.queries[queryKey(p.round, nodeID)] = NewRequestRecord(clock.Now().Unix(), invs)
}

// queryKey returns a string to use for map keys that reprents the given inputs
func queryKey(round int64, nodeID NodeID) string {
	return fmt.Sprintf("%d|%d", round, nodeID)
}
