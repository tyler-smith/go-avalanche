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
	connman *Connman

	round       int64
	targets     map[Hash]Target
	voteRecords map[Hash]*VoteRecord
	nodeIDs     map[NodeID]struct{}
	queries     map[string]RequestRecord

	runMu     sync.Mutex
	isRunning bool
	quitCh    chan (struct{})
	doneCh    chan (struct{})
}

// NewProcessor creates a new *Processor
func NewProcessor(connman *Connman) *Processor {
	return &Processor{
		voteRecords: map[Hash]*VoteRecord{},
		targets:     map[Hash]Target{},
		queries:     map[string]RequestRecord{},
		nodeIDs:     map[NodeID]struct{}{},

		connman: connman,
	}
}

// GetRound returns the current round for the *Processor
func (p *Processor) GetRound() int64 {
	return p.round
}

// AddTargetToReconcile begins the voting process for a given target
// var blah = 0

func (p *Processor) AddTargetToReconcile(t Target) bool {
	if !p.isWorthyPolling(t) {
		return false
	}

	_, ok := p.voteRecords[t.Hash()]
	if ok {
		// fmt.Println("found vr for", t.Hash())
		return false
	}

	// blah++

	// if blah > 10 {
	// 	panic("")
	// }
	// fmt.Println("Adding", t.Hash())
	p.targets[t.Hash()] = t
	p.voteRecords[t.Hash()] = NewVoteRecord(t.IsAccepted())
	return true
}

// RegisterVotes processes responses to queries
func (p *Processor) RegisterVotes(id NodeID, resp Response, updates *[]StatusUpdate) bool {
	// Disabled while hacking on simulations
	if false {
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
			if invs[i].TargetHash != v.GetHash() {
				return false
			}
		}
	}

	votes := resp.GetVotes()

	for _, v := range votes {
		vr, ok := p.voteRecords[v.GetHash()]
		if !ok {
			// We are not voting on this anymore
			continue
		}

		if !p.isWorthyPolling(p.targets[v.GetHash()]) {
			continue
		}

		if !vr.regsiterVote(v.GetError()) {
			// This vote did not provide any extra information
			continue
		}

		// Add appropriate status
		*updates = append(*updates, StatusUpdate{v.GetHash(), vr.status()})

		// When we finalize we want to remove our vote record
		if vr.hasFinalized() {
			// delete(p.voteRecords, v.GetHash())
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

// GetInvsForNextPoll returns Invs for outstanding items that need to be
// resolved by further queries
func (p *Processor) GetInvsForNextPoll() []Inv {
	invs := make([]Inv, 0, len(p.voteRecords))
	for idx, r := range p.voteRecords {
		if r.hasFinalized() {
			// If this has finalized we can just skip.
			continue
		}

		t := p.targets[idx]

		// Obviously do not poll if the target is not worth polling
		if !p.isWorthyPolling(t) {
			continue
		}

		// We don't have a decision, we need more votes.
		invs = append(invs, Inv{t.Type(), idx})
	}

	// sortBlockInvsByWork(invs)

	if len(invs) >= AvalancheMaxElementPoll {
		invs = invs[:AvalancheMaxElementPoll]
	}

	return invs
}

func (p *Processor) GetVoteRecord(h Hash) (*VoteRecord, error) {
	return p.voteRecords[h], nil
}

// getSuitableNodeToQuery returns the best node to send the next query to
func (p *Processor) getSuitableNodeToQuery() NodeID {
	nodeIDs := p.connman.NodesIDs()

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
	invs := p.GetInvsForNextPoll()
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
