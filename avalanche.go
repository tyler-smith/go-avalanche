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

type NodeID int64

const NoNode = NodeID(-1)

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
	Hash(65): &Block{Hash(65), 99, true},
	Hash(66): &Block{Hash(66), 100, true},
}

func blockForHash(h Hash) *Block {
	b, ok := staticTestBlockMap[h]

	// TODO: replace with proper error handling
	if !ok {
		panic("Block not found with hash")
	}

	return b
}

type Inv struct {
	targetType string
	targetHash Hash
}

type Hash int

// Target is is something being decided by consensus; e.g. a transaction or block
type Target interface {
	Hash() Hash

	// Type is the kind of thing; e.g. "transaction" or "block"
	Type() string

	// Score weights to targets against each other; e.g. cumulative work for blocks
	Score() int64

	Valid() bool
}

type Block struct {
	hash  Hash
	work  int64
	valid bool
}

func (b *Block) Hash() Hash {
	return b.hash
}

func (b *Block) Type() string {
	return "block"
}

func (b *Block) Score() int64 {
	return b.work
}

func (b *Block) Valid() bool {
	return b.valid
}

func sortBlockInvsByWork(invs []Inv) {
	blocks := make(blocksByWork, len(invs))
	for i, inv := range invs {
		// TODO: Return error if a targetType is not "block"
		blocks[i] = blockForHash(inv.targetHash)
	}

	sort.Sort(blocks)

	for i, b := range blocks {
		invs[i] = Inv{"block", b.Hash()}
	}
}

type blocksByWork []*Block

func (a blocksByWork) Len() int           { return len(a) }
func (a blocksByWork) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a blocksByWork) Less(i, j int) bool { return a[i].work > a[j].work }
