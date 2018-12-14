package avalanche

type Vote struct {
	err  uint32 // this is called "error" in abc for some reason
	hash Hash
}

func NewVote(err uint32, hash Hash) Vote {
	return Vote{err, hash}
}

func (v Vote) GetHash() Hash {
	return v.hash
}

func (v Vote) GetError() uint32 {
	return v.err
}
