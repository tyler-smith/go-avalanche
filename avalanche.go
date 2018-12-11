package avalanche

import (
	"time"
)

const (
	AvalancheFinalizationScore = 128
	AvalancheTimeStep          = 10 * time.Millisecond
	AvalancheMaxElementPoll    = 4096
)

//
// Development stubs
//

// TODO: replace:
// Block with github.com/gcash/bchutil.Block
// Hash with github.com/gcash/bchd/chaincfg/chainhash.Hash

type blocksByWork []Block

func (a blocksByWork) Len() int           { return len(a) }
func (a blocksByWork) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a blocksByWork) Less(i, j int) bool { return a[i].Work < a[j].Work }

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
