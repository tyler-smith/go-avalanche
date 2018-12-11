package avalanche

import (
	"sync"
	"time"
)

type Processor struct {
	voteRecords map[Hash]*VoteRecord

	runMu     sync.Mutex
	isRunning bool
	quitCh    chan (struct{})
	doneCh    chan (struct{})
}

func NewProcessor() *Processor {
	return &Processor{
		voteRecords: map[Hash]*VoteRecord{},
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
		return vr.isValid()
	}
	return false
}

func (p *Processor) hasFinalized(hash Hash) bool {
	if vr, ok := p.voteRecords[hash]; ok {
		return vr.hasFinalized()
	}
	return false
}

func (p *Processor) registerVotes(resp Response) bool {
	for _, v := range resp.GetVotes() {
		vr, ok := p.voteRecords[v.GetHash()]
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
	}

	sortBlockInvsByWork(invs)

	if len(invs) >= AvalancheMaxElementPoll {
		invs = invs[:AvalancheMaxElementPoll]
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
