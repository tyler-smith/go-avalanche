package avalanche

import "time"

type Poll struct {
	round int64
	invs  []Inv
}

type Response struct {
	round    int64
	cooldown uint32
	votes    []Vote
}

func NewResponse() Response {
	return Response{}
}

func (r Response) GetVotes() []Vote {
	return r.votes
}

func (r Response) GetRound() int64 {
	return r.round
}

type RequestRecord struct {
	timestamp int64
	invs      []Inv
}

func NewRequestRecord(timestamp int64, invs []Inv) RequestRecord {
	return RequestRecord{timestamp, invs}
}

func (r RequestRecord) GetTimestamp() int64 {
	return r.timestamp
}

func (r RequestRecord) GetInvs() []Inv {
	return r.invs
}

func (r RequestRecord) IsExpired() bool {
	return time.Unix(r.timestamp, 0).Add(AvalancheRequestTimeout).Before(clock.Now())
}
