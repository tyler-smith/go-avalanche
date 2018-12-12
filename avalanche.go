package avalanche

import (
	"sort"
	"time"
)

const (
	AvalancheFinalizationScore = 128
	AvalancheTimeStep          = 10 * time.Millisecond
	AvalancheMaxElementPoll    = 4096
)

type Status int

const (
	StatusInvalid Status = iota
	StatusRejected
	StatusAccepted
	StatusFinalized
)

type StatusUpdate struct {
	Hash
	Status
}

var staticTestBlockMap = map[Hash]*Block{
	Hash(65): &Block{Hash(65), 1, 99},
	Hash(66): &Block{Hash(66), 1, 100},
}

func blockForHash(h Hash) *Block {
	b, ok := staticTestBlockMap[h]

	// TODO: replace with proper error handling
	if !ok {
		panic("Block not found with hash")
	}

	return b
}

//
// Development stubs
//

// TODO: replace:
// Block with github.com/gcash/bchutil.Block
// Hash with github.com/gcash/bchd/chaincfg/chainhash.Hash

func sortBlockInvsByWork(invs []Inv) {
	blocks := make(blocksByWork, len(invs))
	for i, inv := range invs {
		// TODO: Return error if a targetType is not "block"
		blocks[i] = blockForHash(inv.targetHash)
	}

	sort.Sort(blocks)

	for i, b := range blocks {
		invs[i] = Inv{"block", b.Hash}
	}
}

type blocksByWork []*Block

func (a blocksByWork) Len() int           { return len(a) }
func (a blocksByWork) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a blocksByWork) Less(i, j int) bool { return a[i].Work > a[j].Work }

type Inv struct {
	targetType string
	targetHash Hash
}

type Block struct {
	Hash   Hash
	Height int
	Work   int
}

type Hash int

type Connman interface {
	Nodes()
	PushMessage()
}

type DummyConnman struct{}

func (DummyConnman) Nodes() {}

func (DummyConnman) PushMessage() {}

var testPeer = NodeID(42)
