package avalanche

type node struct {
	id        NodeID
	avalanche *Processor
	quitCh    chan (struct{})
	doneCh    chan (struct{})
}

func newNode(connman *connman, id NodeID) *node {
	return &node{
		id:        id,
		avalanche: NewProcessor(connman),
		quitCh:    make(chan (struct{})),
		doneCh:    make(chan (struct{})),
	}
}

func (n *node) handleRequest(poll Poll) *Response {
	return n.avalanche.handlePoll(poll)
}

type connman struct {
	nodes map[NodeID]*node
}

func newConnman() *connman {
	return &connman{
		nodes: map[NodeID]*node{},
	}
}

func (c *connman) addNode(id NodeID) {
	c.nodes[id] = newNode(c, id)
}

func (c *connman) nodesIDs() []NodeID {
	nodeIDs := make([]NodeID, 0, len(c.nodes))
	for nodeID, _ := range c.nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}

func (c *connman) sendRequest(id NodeID, poll Poll) *Response {
	node, ok := c.nodes[id]
	if !ok {
		panic("node not found")
	}

	return node.handleRequest(poll)
}
