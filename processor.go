package avalanche

import (
	"fmt"
	"sync"
	"time"
)

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

func NewProcessor(connman *connman) *Processor {
	return &Processor{
		voteRecords: map[Hash]*VoteRecord{},
		queries:     map[string]RequestRecord{},
		nodeIDs:     map[NodeID]struct{}{},

		connman: connman,
	}
}

func (p *Processor) GetRound() int64 {
	return p.round
}

func (p *Processor) AddBlockToReconcile(t Target) bool {
	if !p.isWorthyPolling(t) {
		return false
	}

	_, ok := p.voteRecords[t.Hash()]
	if ok {
		return false
	}

	p.voteRecords[t.Hash()] = NewVoteRecord(true)
	return true
}

func (p *Processor) RegisterVotes(id NodeID, resp Response, updates *[]StatusUpdate) bool {
	key := queryKey(resp.GetRound(), id)
	r, ok := p.queries[key]
	if !ok {
		return false
	}
	delete(p.queries, key)

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

func (p *Processor) IsAccepted(t Target) bool {
	if vr, ok := p.voteRecords[t.Hash()]; ok {
		return vr.isAccepted()
	}
	return false
}

func (p *Processor) GetConfidence(t Target) uint16 {
	vr, ok := p.voteRecords[t.Hash()]
	if !ok {
		panic("VoteRecord not found")
	}

	return vr.getConfidence()
}

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

func (p *Processor) getSuitableNodeToQuery() NodeID {
	nodeIDs := p.connman.nodesIDs()

	if len(nodeIDs) == 0 {
		return NoNode
	}
	return nodeIDs[0]
}

func (p *Processor) isWorthyPolling(t Target) bool {
	return t.Valid()
}

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

func (p *Processor) eventLoop() {
	invs := p.getInvsForNextPoll()
	if len(invs) == 0 {
		return
	}

	nodeID := p.getSuitableNodeToQuery()
	p.queries[queryKey(p.round, nodeID)] = RequestRecord{time.Now().Unix(), invs}
}

func (p *Processor) handlePoll(poll Poll) *Response {
	votes := make([]Vote, 0, len(poll.invs))
	var (
		vr *VoteRecord
		ok bool
	)
	for _, inv := range poll.invs {
		vr, ok = p.voteRecords[inv.targetHash]
		if !ok {
			panic("VoteRecord not found")
		}

		votes = append(votes, Vote{err: uint32(vr.votes), hash: inv.targetHash})
	}

	return &Response{}
}

func queryKey(round int64, nodeID NodeID) string {
	return fmt.Sprintf("%d|%d", round, nodeID)
}
