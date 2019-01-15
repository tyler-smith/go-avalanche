package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gocraft/dbr"
)

var dbConn *dbr.Connection

func createParticipant(db *dbr.Session, p *Participant) error {
	p.Active = true
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	p.ActiveAt = dbr.NewNullTime(p.CreatedAt)

	_, err := db.
		InsertInto("participants").
		Columns("user_id", "key", "active", "updated_at", "active_at").
		Record(p).
		Exec()
	return err
}

func updateParticipantActivity(db *dbr.Session, p *Participant) error {
	p.Active = true
	p.CreatedAt = time.Now()
	p.UpdatedAt = p.CreatedAt
	p.ActiveAt = dbr.NewNullTime(p.CreatedAt)

	_, err := db.
		Update("participants").SetMap(map[string]interface{}{
		"active":     true,
		"created_at": p.CreatedAt,
		"updated_at": p.UpdatedAt,
		"active_at":  p.ActiveAt,
	}).
		Where("id = ?", p.ID).
		Limit(1).
		Exec()

	return err
}

func getParticipants(db *dbr.Session) ([]string, error) {
	keys := []string{}

	_, err := db.
		Select("`key`").
		From("participants").
		Where("active = ?", true).
		// Where("active_at >= ?", time.Now().Add(-5*time.Minute)).
		Load(&keys)

	fmt.Println("keys:", keys)

	return keys, err
}

func createTransaction(db *dbr.Session, t *tx) error {
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt

	_, err := db.
		InsertInto("transactions").
		Columns("id", "created_at", "updated_at").
		Record(t).
		Exec()

	if err != nil && strings.Contains(err.Error(), "Duplicate entry") {
		return nil
	}
	return err
}

func createVoteRecord(db *dbr.Session, vr *voteRecord) error {
	vr.CreatedAt = time.Now()
	vr.UpdatedAt = vr.CreatedAt

	_, err := db.
		InsertInto("vote_records").
		Columns("participant_id", "tx_id", "query_count", "initialized_accepted", "created_at", "updated_at").
		Record(vr).
		Exec()
	return err
}

func finalizeVoteRecord(db *dbr.Session, participantID int, txID string, queryCount int, accepted bool) error {
	_, err := db.
		Update("vote_records").
		Set("query_count", queryCount).
		Set("finalized_accepted", accepted).
		Set("updated_at", time.Now()).
		Set("finalized_at", time.Now()).
		Where("tx_id = ? AND participant_id = ?", txID, participantID).
		Limit(1).
		Exec()
	return err
}
