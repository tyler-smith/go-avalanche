package avalanche

type node struct {
	id NodeID
}

func newNode(id NodeID) *node {
	return &node{id: id}
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
	c.nodes[id] = newNode(id)
}

func (c *connman) nodesIDs() []NodeID {
	nodeIDs := make([]NodeID, 0, len(c.nodes))
	for nodeID, _ := range c.nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}
