package avalanche

type Vote struct {
	err uint32

	// TODO: make this actually a hash
	hash Hash
	// hash [64]byte
}

func NewVote() Vote {
	return Vote{}
}

func (v Vote) GetHash() Hash {
	return v.hash
}

func (v Vote) IsValid() bool {
	return v.err == 0
}
