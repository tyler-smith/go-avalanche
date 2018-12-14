package avalanche

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
