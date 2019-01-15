package main

import (
	"fmt"
	"time"

	"github.com/gocraft/dbr"
	avalanche "github.com/tyler-smith/go-avalanche"
)

type Participant struct {
	ID     int64  `json:"id"`
	UserID int64  `json:"user_id"`
	Key    string `json:"key"`
	Active bool   `json:"active"`

	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	ActiveAt  dbr.NullTime `json:"active_at"`
}

type voteRecord struct {
	ID            int64  `json:"id"`
	TXID          string `json:"tx_id" db:"tx_id"`
	ParticipantID int64  `json:"participant_id"`

	QueryCount          int64 `json:"query_count"`
	InitializedAccepted bool  `json:"initialized_accepted"`
	FinalizedAccepted   bool  `json:"finalized_accepted"`

	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	FinalizedAt dbr.NullTime `json:"finalized_at"`
}

type tx struct {
	isAccepted bool

	ID string `json:"id"`

	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	ConfirmedAt dbr.NullTime `json:"confirmed_at"`
}

func (t *tx) Hash() avalanche.Hash { return avalanche.Hash(t.ID) }

func (t *tx) IsAccepted() bool { return t.isAccepted }

func (*tx) IsValid() bool { return true }

func (*tx) Type() string { return "tx" }

func (*tx) Score() int64 { return 1 }

func debug(str string, args ...interface{}) {
	if debugLoggingEnabled {
		fmt.Println(fmt.Sprintf(str, args...))
	}
}
