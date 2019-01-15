package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gocraft/dbr"
	"github.com/gocraft/health"
	"github.com/gorilla/websocket"
	avalanche "github.com/tyler-smith/go-avalanche"
)

var (
	networkEndpointsMu  sync.RWMutex
	networkEndpoints    []string
	debugLoggingEnabled = true
)

type jsonRPCResponse struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func sniffTransactions(conf BCHDConfig, nodes map[avalanche.NodeID]*node) (chan struct{}, error) {
	c, _, err := websocket.DefaultDialer.Dial("ws://"+conf.Host+"/ws", nil)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}

	// connCfg := &rpcclient.ConnConfig{
	// 	Host:     conf.Host,
	// 	Endpoint: "ws",
	// 	User:     conf.User,
	// 	Pass:     conf.Password,
	// }

	// client, err := rpcclient.New(connCfg, nil)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer client.Shutdown()

	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-done:
				c.Close()
				return
			default:
			}

			_, message, err := c.ReadMessage()
			if err != nil {
				fmt.Println("read:", err)
				return
			}

			go func() {
				resp := &jsonRPCResponse{}
				err = json.Unmarshal(message, resp)
				if err != nil {
					fmt.Println("unmarshal:", err)
					return
				}

				switch resp.Method {
				case "txaccepted":
					h, ok := resp.Params[0].(string)
					if !ok {
						fmt.Println("Failed to convert to string:", resp.Params[0])
						return
					}

					t := &tx{ID: h, isAccepted: true}

					// Add tx to db
					err = createTransaction(dbConn.NewSession(stream), t)
					if err != nil {
						fmt.Println("createTransaction:", err)
						return
					}

					// Send tx to nodes
					for _, n := range nodes {
						n.incoming <- t
					}

					fmt.Println("got tx:", resp.Params[0], "-", time.Now().Unix())
				case "blockconnected":
					return
					h, ok := resp.Params[0].(string)
					if !ok {
						fmt.Println("Failed to convert to string:", resp.Params[0])
						return
					}

					req, err := http.NewRequest("POST", "http://"+conf.Host, bytes.NewBuffer([]byte(`{"jsonrpc":"1.0","id":"0","method":"getblock","params":["`+h+`"]}`)))
					if err != nil {
						log.Fatalln(err)
					}

					req.SetBasicAuth(conf.User, conf.Password)
					resp, err := httpClient.Do(req)
					if err != nil {
						log.Fatalln(err)
					}
					defer resp.Body.Close()

					respBytes, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						log.Fatalln(err)
					}

					txs := struct {
						TX []string `json:"tx"`
					}{}

					err = json.Unmarshal(respBytes, &txs)
					if err != nil {
						log.Fatalln(err)
					}

					for _, tx := range txs.TX {
						fmt.Println("new confirmed tx:", tx)
					}
				default:
					return
				}
			}()
		}
	}()

	err = c.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"1.0","id":"1","method":"authenticate","params":["`+conf.User+`", "`+conf.Password+`"]}`))
	if err != nil {
		return nil, err
	}

	err = c.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"1.0","id":"0","method":"notifynewtransactions","params":[]}`))
	if err != nil {
		return nil, err
	}

	err = c.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"1.0","id":"0","method":"notifyblocks","params":[]}`))
	if err != nil {
		return nil, err
	}

	return done, nil
}

func maintainParticipants(db *dbr.Session) error {
	// func maintainParticipants(rConn redis.Conn) error {
	loadNewEndpoints := func() error {
		networkEndpointsMu.Lock()

		// TODO: get from mysql db
		endpoints, err := getParticipants(db)
		networkEndpointsMu.Unlock()

		if err != nil {
			return err
		}

		networkEndpoints = endpoints
		return nil
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		// ticker := time.NewTicker(redisEndpointTTL)
		for range ticker.C {
			fmt.Println("Refreshing participants...")
			loadNewEndpoints()
		}
	}()

	return loadNewEndpoints()
}

var stream = health.NewStream()

func main() {
	conf, err := NewConfigFromEnv()
	if err != nil {
		log.Fatal(err)
		return
	}

	dbConn, err = dbr.Open("mysql", conf.MySQLDSN, stream)
	if err != nil {
		log.Fatal(err)
		return
	}

	debugLoggingEnabled = conf.Logging

	// Start maintaining pool of participants
	err = maintainParticipants(dbConn.NewSession(stream))
	if err != nil {
		log.Fatal(err)
		return
	}

	// Create nodes
	nodes := make(map[avalanche.NodeID]*node, conf.NodeCount)
	for i := 0; i < conf.NodeCount; i++ {
		id := avalanche.NodeID(i)
		nodes[id] = newNode(id, avalanche.NewConnman())
		nodes[id].start()
	}

	// Listen for new txs to the mempool and attempt to finalize
	stopSniffing, err := sniffTransactions(conf.BCHD, nodes)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Wait for exit signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	debug("Shutting down...")

	close(stopSniffing)

	// Stop all nodes
	for _, n := range nodes {
		n.stop()
	}

	debug("Done shutting down")
}

type node struct {
	id avalanche.NodeID

	snowball   *avalanche.Processor
	snowballMu *sync.RWMutex

	participant *Participant

	incoming chan (*tx)
	host     string

	quitCh chan (struct{})
	doneWg *sync.WaitGroup
}

func newNode(id avalanche.NodeID, connman *avalanche.Connman) *node {
	return &node{
		id:       id,
		incoming: make(chan (*tx), 10),

		snowballMu: &sync.RWMutex{},
		snowball:   avalanche.NewProcessor(connman),

		quitCh: make(chan (struct{})),
		doneWg: &sync.WaitGroup{},
	}
}

func (n *node) start() {
	n.doneWg.Add(3)
	n.startProcessor()
	n.startIntake()
	err := n.startPollServer()
	if err != nil {
		n.doneWg.Done()
		panic(err)
	}
}

func (n *node) stop() {
	close(n.quitCh)
	n.doneWg.Wait()
}

func (n *node) startProcessor() {
	go func() {
		defer n.doneWg.Done()

		var (
			queries = 0
			ticker  = time.NewTicker(avalanche.AvalancheTimeStep)
		)

		for i := 0; ; i++ {
			select {
			case <-n.quitCh:
				return
			case <-ticker.C:
			}

			networkEndpointsMu.RLock()
			if len(networkEndpoints) == 0 {
				networkEndpointsMu.RUnlock()
				continue
			}
			endpoint := networkEndpoints[i%len(networkEndpoints)]
			networkEndpointsMu.RUnlock()

			// Don't query ourself
			if endpoint == n.host {
				continue
			}

			// Get invs for next query
			updates := []avalanche.StatusUpdate{}
			n.snowballMu.Lock()
			invs := n.snowball.GetInvsForNextPoll()
			n.snowballMu.Unlock()

			if len(invs) == 0 {
				continue
			}

			// Query next node
			// fmt.Println("sending query...")
			resp, err := n.sendQuery(endpoint, invs)
			if err != nil {
				continue
			}

			// Register query response
			queries++
			n.snowballMu.Lock()
			n.snowball.RegisterVotes(n.id, *resp, &updates)
			n.snowballMu.Unlock()

			// Nothing interesting happened; go to next cycle
			if len(updates) == 0 {
				continue
			}

			// Got some updates; process them
			for _, update := range updates {
				if update.Status == avalanche.StatusFinalized {
					err = finalizeVoteRecord(dbConn.NewSession(stream), int(n.id), string(update.Hash), queries, true)
					debug("Finalized tx %s on node %d on query %d - %d", update.Hash, n.id, queries, time.Now().Unix())
				} else if update.Status == avalanche.StatusAccepted {
					debug("Accepted tx %s on node %d on query %d", update.Hash, n.id, queries)
				} else if update.Status == avalanche.StatusRejected {
					err = finalizeVoteRecord(dbConn.NewSession(stream), int(n.id), string(update.Hash), queries, false)
					debug("Rejected tx %s on node %d on query %d", update.Hash, n.id, queries)
				} else if update.Status == avalanche.StatusInvalid {
					debug("Invalidated tx %s on node %d on query %d", update.Hash, n.id, queries)
				} else {
					fmt.Println(update.Status == avalanche.StatusAccepted)
					panic(update)
				}
			}
		}
	}()
}

// startIntake adds incoming txs to Processor
func (n *node) startIntake() {
	go func() {
		defer n.doneWg.Done()
		var err error
		for {
			select {
			case <-n.quitCh:
				return
			case t := <-n.incoming:
				fmt.Println("tx intake", *t)
				err = createVoteRecord(dbConn.NewSession(stream), &voteRecord{
					TXID:                t.ID,
					ParticipantID:       n.participant.ID,
					InitializedAccepted: true,
				})

				if err != nil && !strings.Contains(err.Error(), "Duplicate entry") {
					debug("Error creating vote record: %v", err)
					continue
				}

				fmt.Println("adding tx to snowball")
				n.snowballMu.Lock()
				n.snowball.AddTargetToReconcile(t)
				n.snowballMu.Unlock()
			}
		}
	}()
}

func (n *node) startPollServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", n.respondToPoll)

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}

	n.host = "http://localhost:" + strconv.Itoa(l.Addr().(*net.TCPAddr).Port)

	n.participant = &Participant{UserID: 42, Key: n.host}
	err = createParticipant(dbConn.NewSession(stream), n.participant)
	if err != nil {
		return err
	}

	fmt.Println("id:", n.participant.ID)

	go func() {
		defer n.doneWg.Done()
		fmt.Println("Node", n.id, "listening at", n.host)

		srv := http.Server{Handler: mux}
		go srv.Serve(l)

		ticker := time.NewTicker(3 * time.Minute)

	PARTICIPANT_UPDATE_LOOP:
		for {
			select {
			case <-n.quitCh:
				break PARTICIPANT_UPDATE_LOOP
			case <-ticker.C:
				err = updateParticipantActivity(dbConn.NewSession(stream), participant)
				if err != nil {
					fmt.Println(err)
				}

			}
		}

		fmt.Println("stopping poll server")
		srv.Shutdown(nil)
	}()

	return nil
}

func (n node) sendQuery(endpoint string, invs []avalanche.Inv) (*avalanche.Response, error) {
	body, err := json.Marshal(invs)
	if err != nil {
		return nil, err
	}

	httpResp, err := http.Post(endpoint, "text", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBytes, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	resp := &avalanche.Response{}
	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (n *node) respondToPoll(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("Error reading body: %v\n", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	invs := []avalanche.Inv{}
	err = json.Unmarshal(body, &invs)
	if err != nil {
		fmt.Printf("Error unmarshalling body: %v\n", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	votes := make([]avalanche.Vote, len(invs))

	n.snowballMu.Lock()
	defer n.snowballMu.Unlock()

	for i := 0; i < len(invs); i++ {
		n.snowball.AddTargetToReconcile(&tx{
			ID:         string(invs[i].TargetHash),
			isAccepted: true,
		})

		votes[i] = avalanche.NewVote(0, invs[i].TargetHash)
	}

	resp := avalanche.NewResponse(0, 0, votes)
	body, err = json.Marshal(&resp)
	if err != nil {
		fmt.Printf("Error marshalling response: %v\n", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	fmt.Fprintln(w, string(body))
}
