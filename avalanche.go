package avalanche

import (
	"sort"
	"time"
)

const (
	// AvalancheFinalizationScore is the confidence score we consider to be final
	AvalancheFinalizationScore = 128

	// AvalancheTimeStep is the amount of time to wait between event ticks
	AvalancheTimeStep = 10 * time.Millisecond

	// AvalancheMaxElementPoll is the maximum number of invs to send in a single
	// query
	AvalancheMaxElementPoll = 4096

	// AvalancheRequestTimeout is the amount of time to wait for a response to a
	// query
	AvalancheRequestTimeout = 1 * time.Minute
)

// NodeID is the identifier for an avalanche node
type NodeID int64

// NoNode represents no suitable nodes available
const NoNode = NodeID(-1)

type nodesInRequestOrder []NodeID

// Len implements the sort interface Len method for nodesInRequestOrder
func (a nodesInRequestOrder) Len() int { return len(a) }

// Swap implements the sort interface Swap method for nodesInRequestOrder
func (a nodesInRequestOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// Less implements the sort interface Less method for nodesInRequestOrder
func (a nodesInRequestOrder) Less(i, j int) bool { return a[i] < a[j] }

// Status is the status of consensus on a particular target
type Status int

const (
	// StatusInvalid means the target is invalid
	StatusInvalid Status = iota

	// StatusRejected means the target is been deemed to be rejected
	StatusRejected

	// StatusAccepted means the target is been deemed to be accepted
	StatusAccepted

	// StatusFinalized means the consensus on the target is been finalized
	StatusFinalized
)

// StatusUpdate represents a change in status for a particular Target
type StatusUpdate struct {
	Hash
	Status
}

// Inv is a poll request for a Target
type Inv struct {
	targetType string
	targetHash Hash
}

// Hash is a unique digest that represents a Target
type Hash int

// Target is is something being decided by consensus; e.g. a transaction or block
type Target interface {
	// Hash returns the digest used as an ID for the Target
	Hash() Hash

	// Type is the kind of thing; e.g. "transaction" or "block"
	Type() string

	// IsAccepted returns whether or not the target should be considered accepted
	// when first being considered
	IsAccepted() bool

	// Score weights to targets against each other; e.g. cumulative work for blocks
	Score() int64

	// IsValid returns whether or not the Target is still valid
	// If a Target becomes invalid we'll stop polling for it
	IsValid() bool
}

// clock allows access to the current time
// It can be swapped out with a stub for testing
var clock clocker = realClocker{}

// clocker returns the current time
type clocker interface{ Now() time.Time }

type realClocker struct{}

// Now returns the current time
func (realClocker) Now() time.Time { return time.Now() }

type stubClocker struct{ t time.Time }

// Now returns the stub's preset time
func (c stubClocker) Now() time.Time { return c.t }

//
// Block stubs
//
var staticTestBlockMap = map[Hash]*Block{
	Hash(65): {Hash(65), 99, true, true},
	Hash(66): {Hash(66), 100, true, false},
}

func blockForHash(h Hash) *Block {
	b, ok := staticTestBlockMap[h]

	// TODO: replace with proper error handling
	if !ok {
		panic("Block not found with hash")
	}

	return b
}

// Block is a stub for Bitcoin block
type Block struct {
	hash            Hash
	work            int64
	valid           bool
	isInActiveChain bool
}

// Hash returns the Blocks id
func (b *Block) Hash() Hash {
	return b.hash
}

// Type returns the Target type; in this case a block
func (b *Block) Type() string {
	return "block"
}

// Score returns the weight of the block against others; in the this the work
func (b *Block) Score() int64 {
	return b.work
}

// IsAccepted returns whether or not the Block has been accepted
func (b *Block) IsAccepted() bool {
	return b.isInActiveChain
}

// IsValid returns whether or not the Block is valid
func (b *Block) IsValid() bool {
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

// Len implements the sort interface's Len method for blocksByWork
func (a blocksByWork) Len() int { return len(a) }

// Swap implements the sort interface's Swap method for blocksByWork
func (a blocksByWork) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// Less implements the sort interface's Less method for blocksByWork
func (a blocksByWork) Less(i, j int) bool { return a[i].work > a[j].work }
