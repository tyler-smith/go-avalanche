package avalanche

type node struct {
	id   NodeID
	host string
}

type Connman struct {
	nodes map[NodeID]*node
}

func NewConnman() *Connman {
	return &Connman{
		nodes: map[NodeID]*node{},
	}
}

func (c *Connman) AddNode(id NodeID, host string) {
	c.nodes[id] = &node{id: id, host: host}
}

func (c *Connman) NodesIDs() []NodeID {
	nodeIDs := make([]NodeID, 0, len(c.nodes))
	for nodeID := range c.nodes {
		nodeIDs = append(nodeIDs, nodeID)
	}
	return nodeIDs
}
