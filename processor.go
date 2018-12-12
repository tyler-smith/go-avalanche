package avalanche

import (
	"sync"
	"time"
)

type NodeID int64

type RequestRecord struct {
	timestamp int64
	invs      []Inv
}

func NewRequestRecord(timestamp int64, invs []Inv) RequestRecord {
	return RequestRecord{timestamp, invs}
}

func (r RequestRecord) GetTimestamp() int64 {
	return r.timestamp
}

func (r RequestRecord) GetInvs() []Inv {
	return r.invs
}

type Processor struct {
	voteRecords map[Hash]*VoteRecord
	queries     map[NodeID]RequestRecord
	nodeIDs     map[NodeID]struct{}

	runMu     sync.Mutex
	isRunning bool
	quitCh    chan (struct{})
	doneCh    chan (struct{})
}

func NewProcessor() *Processor {
	return &Processor{
		voteRecords: map[Hash]*VoteRecord{},
		queries:     map[NodeID]RequestRecord{},
		nodeIDs:     map[NodeID]struct{}{},
	}
}

func (p *Processor) addBlockToReconcile(hash Hash) bool {
	_, ok := p.voteRecords[hash]
	if ok {
		return false
	}

	p.voteRecords[hash] = NewVoteRecord()
	return true
}

func (p *Processor) isAccepted(hash Hash) bool {
	if vr, ok := p.voteRecords[hash]; ok {
		return vr.isAccepted()
	}
	return false
}

func (p *Processor) registerVotes(id NodeID, resp Response, updates *[]StatusUpdate) bool {
	r, ok := p.queries[id]
	if !ok {
		return false
	}
	delete(p.queries, id)

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

		if !vr.regsiterVote(v.IsValid()) {
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

func (p *Processor) getInvsForNextPoll() []Inv {
	invs := make([]Inv, 0, len(p.voteRecords))

	for idx, r := range p.voteRecords {
		if r.hasFinalized() {
			// If this has finalized we can just skip.
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
	return testPeer
	// return random node from map of fake nodes
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
	p.queries[nodeID] = RequestRecord{time.Now().Unix(), invs}

	// TODO: send to node
}
