package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"time"

	avalanche "github.com/tyler-smith/go-avalanche"
)

const (
	nodeCount = 1e2
	txCount   = 1e2
)

var (
	networkNodes   []*node
	loggingEnabled = true
)

func main() {
	logging := flag.Bool("logging", false, "Enable logging")
	flag.Parse()

	if logging != nil {
		loggingEnabled = *logging
	}

	// Create nodes
	networkNodes = make([]*node, nodeCount)
	for i := 0; i < nodeCount; i++ {
		networkNodes[i] = newNode(avalanche.NodeID(i), avalanche.NewConnman())
	}

	// Create wg with a slot for each node
	wg := &sync.WaitGroup{}
	wg.Add(nodeCount)

	// Start node processing with wg to signal completion
	for i := 0; i < nodeCount; i++ {
		go networkNodes[i].run(wg)
	}

	t0 := time.Now()

	// Send txs to each node
	for _, t := range rand.Perm(txCount) {
		for i := 0; i < nodeCount; i++ {
			networkNodes[i].incoming <- &tx{hash: int64(t), isAccepted: true}
		}
	}

	// Stop all nodes
	for i := 0; i < nodeCount; i++ {
		close(networkNodes[i].incoming)
	}

	// Wait for all nodes to finish
	wg.Wait()

	fmt.Println(fmt.Sprintf("Finished in %fs", time.Now().Sub(t0).Seconds()))
	log("Nodes fully finalized: %d", nodesFullyFinalized)
}

func log(str string, args ...interface{}) {
	if loggingEnabled {
		fmt.Println(fmt.Sprintf(str, args...))
	}
}

type node struct {
	id         avalanche.NodeID
	snowball   *avalanche.Processor
	snowballMu *sync.RWMutex
	incoming   chan (*tx)
}

func newNode(id avalanche.NodeID, connman *avalanche.Connman) *node {
	return &node{
		id:         id,
		snowball:   avalanche.NewProcessor(connman),
		snowballMu: &sync.RWMutex{},
		incoming:   make(chan (*tx), 10),
	}
}

var nodesFullyFinalized = 0

func (n node) run(wg *sync.WaitGroup) {
	defer wg.Done()

	// Create goroutine to add incoming txs to Processor
	doneAdding := false
	go func() {
		for t := range n.incoming {
			n.snowballMu.Lock()
			n.snowball.AddTargetToReconcile(t)
			n.snowballMu.Unlock()
		}
		doneAdding = true
	}()

	// Start query/response event loop
	var (
		queries        = 0
		finalizedCount = 0
	)
	for i := 0; i < 1e8; i++ {
		nodeID := i % len(networkNodes)

		// Don't query ourself
		if nodeID == int(n.id) {
			continue
		}

		// Get invs for next query
		queries++
		updates := []avalanche.StatusUpdate{}
		n.snowballMu.Lock()
		invs := n.snowball.GetInvsForNextPoll()
		n.snowballMu.Unlock()

		if len(invs) == 0 {
			log("Out of invs: %d", n.id)
			time.Sleep(avalanche.AvalancheTimeStep)
			continue
		}

		// Query next node
		resp := networkNodes[nodeID].query(invs)

		// Register query response
		n.snowballMu.Lock()
		n.snowball.RegisterVotes(n.id, resp, &updates)
		n.snowballMu.Unlock()

		// Nothing interesting happened; go to next cycle
		if len(updates) == 0 {
			continue
		}

		// Got some updates; process them
		for _, update := range updates {
			if update.Status == avalanche.StatusFinalized {
				finalizedCount++
				log("Finalized tx %d on node %d after %d queries", update.Hash, n.id, queries)
			} else if update.Status == avalanche.StatusAccepted {
				log("Accepted tx %d on node %d after %d queries", update.Hash, n.id, queries)
			} else if update.Status == avalanche.StatusRejected {
				log("Rejected tx %d on node %d after %d queries", update.Hash, n.id, queries)
			} else if update.Status == avalanche.StatusInvalid {
				log("Invalidated tx %d on node %d after %d queries", update.Hash, n.id, queries)
			} else {
				fmt.Println(update.Status == avalanche.StatusAccepted)
				panic(update)
			}
		}

		// If we're done accepting new tx and have finalized all outstanding txs
		// then we're finished
		if doneAdding && finalizedCount >= txCount {
			nodesFullyFinalized++
			return
		}
	}

	log("Limit exceeded")
}

func (n node) query(invs []avalanche.Inv) avalanche.Response {
	n.snowballMu.Lock()
	defer n.snowballMu.Unlock()

	votes := make([]avalanche.Vote, len(invs))

	for i := 0; i < len(invs); i++ {
		t := &tx{hash: int64(invs[i].TargetHash), isAccepted: true}

		n.snowball.AddTargetToReconcile(t)

		var vote uint32 = 0
		if !n.snowball.IsAccepted(t) {
			vote = 1
		}

		// Randomly flip votes to prolong convergence
		// if rand.Float64()*100 < 30 {
		// 	vote = vote ^ 1
		// }

		votes[i] = avalanche.NewVote(vote, invs[i].TargetHash)
	}

	return avalanche.NewResponse(0, 0, votes)
}

// tx
type tx struct {
	hash       int64
	isAccepted bool
}

func (t *tx) Hash() avalanche.Hash { return avalanche.Hash(t.hash) }

func (t *tx) IsAccepted() bool { return t.isAccepted }

func (*tx) IsValid() bool { return true }

func (*tx) Type() string { return "tx" }

func (*tx) Score() int64 { return 1 }
