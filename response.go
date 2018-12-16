package avalanche

import "time"

// Response is a list of votes that respond to a Poll
type Response struct {
	round    int64
	cooldown uint32
	votes    []Vote
}

// NewResponse creates a new Response object with the given votes
func NewResponse(round int64, cooldown uint32, votes []Vote) Response {
	return Response{round, cooldown, votes}
}

// GetVotes returns the votes in the Response
func (r Response) GetVotes() []Vote {
	return r.votes
}

// GetRound returns the round of the Response
func (r Response) GetRound() int64 {
	return r.round
}

// RequestRecord is a poll request for more votes
type RequestRecord struct {
	timestamp int64
	invs      []Inv
}

// NewRequestRecord creates a new RequestRecord
func NewRequestRecord(timestamp int64, invs []Inv) RequestRecord {
	return RequestRecord{timestamp, invs}
}

// GetTimestamp returns the timestamp that the request was created
func (r RequestRecord) GetTimestamp() int64 {
	return r.timestamp
}

// GetInvs returns the poll Invs for the request
func (r RequestRecord) GetInvs() []Inv {
	return r.invs
}

// IsExpired returns true if the request has expired
func (r RequestRecord) IsExpired() bool {
	return time.Unix(r.timestamp, 0).Add(AvalancheRequestTimeout).Before(clock.Now())
}
