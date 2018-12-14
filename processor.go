package avalanche

import (
	"fmt"
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
	round       int64
	queries     map[string]RequestRecord
	// queries     map[NodeID]RequestRecord
	nodeIDs map[NodeID]struct{}

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

func (p *Processor) addBlockToReconcile(b *Block) bool {
	if !p.isWorthyPolling(b) {
		return false
	}

	_, ok := p.voteRecords[b.Hash]
	if ok {
		return false
	}

	p.voteRecords[b.Hash] = NewVoteRecord(true)
	return true
}

func (p *Processor) isAccepted(b *Block) bool {
	if vr, ok := p.voteRecords[b.Hash]; ok {
		return vr.isAccepted()
	}
	return false
}

func queryKey(round int64, nodeID NodeID) string {
	return fmt.Sprintf("%d|%d", round, nodeID)
}

func (p *Processor) registerVotes(id NodeID, resp Response, updates *[]StatusUpdate) bool {
	// fmt.Println("looking for query for", id)
	key := queryKey(resp.GetRound(), id)
	r, ok := p.queries[key]
	if !ok {
		fmt.Println("did not find query for", id)
		return false
	}
	fmt.Println("deleting query 0")
	delete(p.queries, key)

	// _nextNodeToQuery++
	// fmt.Println("incremented next node")
	// fmt.Println("found query for", id)

	invs := r.GetInvs()
	votes := resp.GetVotes()

	if len(votes) != len(invs) {
		fmt.Println("wrong len:", len(votes), len(invs))
		return false
	}

	for i, v := range votes {
		if invs[i].targetHash != v.GetHash() {
			fmt.Println("wrong hash:", invs[i].targetHash, v.GetHash())
			return false
		}
	}
	// fmt.Println("processing votes", votes)
	for _, v := range votes {
		// fmt.Println("vote for:", v.GetHash(), v.GetError())

		vr, ok := p.voteRecords[v.GetHash()]
		if !ok {
			fmt.Println("no longer voting on this")
			// We are not voting on this anymore
			continue
		}

		fmt.Println(v.GetHash())
		if !vr.regsiterVote(v.GetError()) {
			// fmt.Println("no extra data")
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

		fmt.Println("adding status:", status)

		*updates = append(*updates, StatusUpdate{v.GetHash(), status})

		// When we finalize we want to remove our vote record
		if finalized {
			delete(p.voteRecords, v.GetHash())
		}
	}

	p.nodeIDs[id] = struct{}{}

	return true
}

func (p *Processor) getConfidence(b *Block) uint16 {
	vr, ok := p.voteRecords[b.Hash]
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

		// Obviously do not poll if the block is not worth polling
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

// var _nextNodeToQuery = 0

const NoNode = NodeID(-1)

func (p *Processor) getSuitableNodeToQuery() NodeID {
	// return NodeID(0)
	nodeIDs := p.connman.nodesIDs()

	if len(nodeIDs) == 0 {
		return NoNode
	}
	return nodeIDs[0]
	// fmt.Println("nodeid:", nodeIDs[_nextNodeToQuery%len(nodeIDs)])
	// return nodeIDs[_nextNodeToQuery%len(nodeIDs)]
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
	// fmt.Println("created query for", nodeID)
	// Send to node and handle response
	// In a real situation the send and receive will be async
	// resp := p.connman.sendRequest(nodeID, Poll{
	// 	round: p.round,
	// 	invs:  invs,
	// })

	// p.registerVotes(nodeID, *resp, &[]StatusUpdate{})
}

func (p *Processor) handlePoll(poll Poll) *Response {
	// TODO: figure out the 16bit vs 32bit discrepency

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

func (p *Processor) isWorthyPolling(b *Block) bool {
	return b.Valid
}
