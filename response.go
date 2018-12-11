package avalanche

type Response struct {
	cooldown uint32
	votes    []Vote
}

func NewResponse() Response {
	return Response{}
}

func (r Response) GetVotes() []Vote {
	return r.votes
}
