package avalanche

import (
	"sync"
	"time"
)

const (
	AvalancheFinalizationScore = 128
	// AvalancheFinalizationScore uint16 = 128

	AvalancheTimeStep = 10 * time.Millisecond

	AvalancheMaxElementPoll = 4096
)

type VoteRecord struct {
	votes      uint16
	confidence uint16
}

func NewVoteRecord() *VoteRecord {
	return &VoteRecord{votes: 0xaaaa}
}

func (vr VoteRecord) isValid() bool {
	return (vr.confidence & 0x01) == 1
}

func (vr VoteRecord) getConfidence() uint16 {
	return vr.confidence >> 1
}

func (vr VoteRecord) hasFinalized() bool {
	return vr.getConfidence() >= AvalancheFinalizationScore
}

func (vr *VoteRecord) regsiterVote(vote bool) bool {
	var voteInt uint16
	if vote {
		voteInt = 1
	}

	vr.votes = (vr.votes << 1) | voteInt

	bitCount := countBits(vr.votes & 0xff)
	yes := (bitCount > 6)
	no := (bitCount < 2)

	// Vote is inconclusive
	if !yes && !no {
		return false
	}

	// Vote is conclusive and agrees with our current state
	if vr.isValid() == yes {
		vr.confidence += 2

		// Vote is conclusive but does not agree with our current state
	} else {
		vr.confidence = 0
		if yes {
			vr.confidence++
		}
	}

	return true
}

func countBits(i uint16) (count int) {
	for ; i > 0; i &= (i - 1) {
		count++
	}
	return count
}

type Vote struct {
	err uint32

	// TODO: make this actually a hash
	hash blockIndex
	// hash [64]byte
}

func NewVote() Vote {
	return Vote{}
}

func (v Vote) GetHash() blockIndex {
	return v.hash
}

func (v Vote) IsValid() bool {
	return v.err == 0
}

type Response struct {
	cooldown uint32
	votes    []Vote
}

func NewResponse() Response {
	return Response{}
}

func (r Response) GetVotes() []Vote {
	return r.votes
}

type Processor struct {
	voteRecords map[blockIndex]*VoteRecord

	runMu     sync.Mutex
	isRunning bool
	quitCh    chan (struct{})
	doneCh    chan (struct{})
}

func NewProcessor() *Processor {
	return &Processor{
		voteRecords: map[blockIndex]*VoteRecord{},
	}
}

func (p *Processor) addBlockToReconcile(index blockIndex) bool {
	_, ok := p.voteRecords[index]
	if ok {
		return false
	}

	p.voteRecords[index] = NewVoteRecord()
	return true
}

func (p *Processor) isAccepted(index blockIndex) bool {
	if vr, ok := p.voteRecords[index]; ok {
		return vr.isValid()
	}
	return false
}

func (p *Processor) hasFinalized(index blockIndex) bool {
	if vr, ok := p.voteRecords[index]; ok {
		return vr.hasFinalized()
	}
	return false
}

func (p *Processor) registerVotes(resp Response) bool {
	for _, v := range resp.GetVotes() {
		vr, ok := p.voteRecords[blockIndexForHash(v.GetHash())]
		if !ok {
			// We are not voting on this anymore
			continue
		}

		vr.regsiterVote(v.IsValid())
	}

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
		if len(invs) >= AvalancheMaxElementPoll {
			break
		}
	}

	return invs
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
				// Perform loop code
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

//
// Development stubs
//
type Inv struct {
	targetType string
	targetHash blockIndex
}

type blockIndex int

// TODO: figure out the best way to represent or abstract blocks
func blockIndexForHash(hash blockIndex) blockIndex {
	return hash
}
