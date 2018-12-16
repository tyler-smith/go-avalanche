package avalanche

import (
	"time"
)

const (
	AvalancheFinalizationScore = 128
	AvalancheTimeStep          = 10 * time.Millisecond
	AvalancheMaxElementPoll    = 4096
	AvalancheRequestTimeout    = 1 * time.Minute
)

// NodeID is the identifier for an avalanche node
type NodeID int64

const NoNode = NodeID(-1)

type nodesInRequestOrder []NodeID

func (a nodesInRequestOrder) Len() int           { return len(a) }
func (a nodesInRequestOrder) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a nodesInRequestOrder) Less(i, j int) bool { return a[i] < a[j] }

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

type Inv struct {
	targetType string
	targetHash Hash
}

// Hash is a unique digest that represents a Target
type Hash int

// Target is is something being decided by consensus; e.g. a transaction or block
type Target interface {
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

func (realClocker) Now() time.Time { return time.Now() }

type stubClocker struct{ t time.Time }

func (c stubClocker) Now() time.Time { return c.t }
