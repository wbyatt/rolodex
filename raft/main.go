package raft

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/google/uuid"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
)

type ServerState int

const (
	LEADER ServerState = iota
	FOLLOWER
	CANDIDATE
	PENDING
)

type RaftServer struct {
	RolodexRaftServer

	id            string
	peerThreshold int

	localAddress  string
	peerAddresses []string
	peerClients   []RolodexRaftClient
	port          int

	state       ServerState
	heartbeat   chan bool
	currentTerm int32
	votedFor    string
}

func NewRaftServer() *RaftServer {
	return &RaftServer{
		id:            uuid.New().String(),
		peerThreshold: 2,

		localAddress:  "localhost",
		peerAddresses: make([]string, 0),

		state:       PENDING,
		heartbeat:   make(chan bool),
		currentTerm: 0,
		votedFor:    "",
	}
}

func (raftServer *RaftServer) interceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	method := info.FullMethod

	peer, _ := peer.FromContext(ctx)
	addrLocal, _, _ := net.SplitHostPort(peer.LocalAddr.String())
	addrPeer, _, _ := net.SplitHostPort(peer.Addr.String())

	log.Printf("%v received request for %v from %v", addrLocal, method, addrPeer)
	// Update local address if we got a request from a different node
	raftServer.localAddress = addrLocal

	// Update peer list if we got a request from a different node
	raftServer.addPeer(addrPeer)

	// Block further action if we are not ready
	if raftServer.state == PENDING && method != "/raft.RolodexRaft/Discover" && method != "/raft.RolodexRaft/RequestVote" {
		return nil, fmt.Errorf("server not ready, still in PENDING state")
	}

	// Proceed
	return handler(ctx, req)
}

func StartServer(port *int, signals chan bool) *RaftServer {
	listener, error := net.Listen("tcp", fmt.Sprintf(":%d", *port))

	if error != nil {
		panic(error)
	}

	raftServer := NewRaftServer()
	raftServer.port = *port

	grpcServer := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()),
		grpc.UnaryInterceptor(raftServer.interceptor),
	)

	RegisterRolodexRaftServer(grpcServer, raftServer)

	reflection.Register(grpcServer)

	log.Printf("Starting Raft server at %v", listener.Addr())

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
			signals <- false
		}

		signals <- true
	}()

	return raftServer
}

func (raftServer *RaftServer) StartRaft() {
	raftServer.state = FOLLOWER

	raftServer.startTermTimeout()
}

func uglyHelper(timeout chan bool) context.CancelFunc {
	randomTimeout := time.Duration((rand.Intn(1500) + 1500)) * time.Millisecond
	ctx, cancelFunc := context.WithCancel(context.Background())

	go func() {
		time.Sleep(randomTimeout)

		select {
		case <-ctx.Done():
			return
		default:
			timeout <- true
		}
	}()

	return cancelFunc
}

func (raftServer *RaftServer) startTermTimeout() {
	timeout := make(chan bool)
	defer close(timeout)

	cancelFunc := uglyHelper(timeout)

	for {
		select {
		case <-raftServer.heartbeat:
			cancelFunc()
			cancelFunc = uglyHelper(timeout)

		case trigger := <-timeout:
			if trigger {
				raftServer.becomeCandidate()
			}
			return
		}
	}
}

func (raftServer *RaftServer) becomeCandidate() {
	log.Printf("Node %v is becoming a candidate", raftServer.localAddress)
	raftServer.state = CANDIDATE
	raftServer.currentTerm++
	raftServer.votedFor = raftServer.localAddress

	voteTally := 0
	voteThreshold := len(raftServer.peerAddresses)/2 + 1
	voteChannel := make(chan bool)
	// timeoutChannel := make(chan bool)
	defer close(voteChannel)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)

	go func(ctx context.Context) {
		// From 1.5 to 3.0 seconds
		randomTimeout := time.Duration((rand.Intn(1500) + 1500)) * time.Millisecond
		time.Sleep(randomTimeout)

		select {
		case <-ctx.Done():
			return
		default:
			if raftServer.state == CANDIDATE {
				raftServer.becomeCandidate()
			}
		}
	}(ctx)

	for _, peer := range raftServer.peerClients {
		go func(peer RolodexRaftClient) {
			voteResponse, err := peer.RequestVote(context.Background(), &RequestVoteRequest{
				Term:         raftServer.currentTerm,
				CandidateId:  raftServer.localAddress,
				LastLogIndex: 0,
				LastLogTerm:  0,
			})

			if err != nil {
				log.Printf("Failed to request vote: %v", err)
				return
				// voteChannel <- false
			}

			voteChannel <- voteResponse.VoteGranted
		}(peer)
	}

	for {
		select {
		case vote := <-voteChannel:
			if vote {
				voteTally++
			}

			if voteTally >= voteThreshold {
				// close(timeoutChannel)
				cancelFunc()

				raftServer.becomeLeader()
				return
			}
		}

	}
}

func (raftServer *RaftServer) becomeLeader() {
	raftServer.state = LEADER

	for _, peer := range raftServer.peerClients {
		go func(peer RolodexRaftClient) {
			for {
				_, err := peer.AppendEntries(context.Background(), &AppendEntriesRequest{
					Term:         raftServer.currentTerm,
					LeaderId:     raftServer.localAddress,
					PrevLogIndex: 0,
					PrevLogTerm:  0,
					Entries:      nil,
					LeaderCommit: 0,
				})

				if err != nil {
					log.Printf("Failed to send heartbeat: %v", err)
				}

				time.Sleep(1000 * time.Millisecond)
			}
		}(peer)
	}
}

func (raftServer *RaftServer) addPeer(candidatePeer string) {
	// we are not adding ourself to the peer list
	if candidatePeer == raftServer.localAddress {
		return
	}

	// we are only adding peers that we don't already have
	for _, peer := range raftServer.peerAddresses {
		if peer == candidatePeer {
			return
		}
	}

	raftServer.peerAddresses = append(raftServer.peerAddresses, candidatePeer)
}

func (raftServer *RaftServer) StartDiscovery(discoveryUrl *string) {
	conn, err := grpc.NewClient(*discoveryUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	discoveryClient := NewRolodexRaftClient(conn)

	discoveryRequest := &DiscoverRequest{
		Members: raftServer.peerAddresses,
	}

	discoveryResponse, err := discoveryClient.Discover(context.Background(), discoveryRequest)

	if err != nil {
		log.Fatalf("Failed to discover: %v", err)
	}

	raftServer.localAddress = discoveryResponse.Caller

	// Add all the peers we discovered
	for _, member := range discoveryResponse.Members {
		raftServer.addPeer(member)
	}

	// If we don't have enough peers, we retry
	if len(raftServer.peerAddresses) < raftServer.peerThreshold {
		randomWait := 150 + rand.Intn(150)
		time.Sleep(time.Duration(randomWait) * time.Millisecond)
		raftServer.StartDiscovery(discoveryUrl)

		return
	}

	// We end discovery if we have enough peers
	log.Printf("Discovery complete:")
	log.Printf("\tSelf: %v", raftServer.localAddress)
	log.Printf("\tPeers: %v", raftServer.peerAddresses)

	for _, peer := range raftServer.peerAddresses {
		peerAddress := fmt.Sprintf("%v:%v", peer, raftServer.port)
		conn, err := grpc.NewClient(peerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))

		if err != nil {
			log.Fatalf("Failed to connect: %v", err)
		}

		raftServer.peerClients = append(raftServer.peerClients, NewRolodexRaftClient(conn))
	}

	return
}

func Main(port *int, discoveryUrl *string) {
	signals := make(chan bool)
	defer close(signals)

	raftServer := StartServer(port, signals)
	raftServer.StartDiscovery(discoveryUrl)
	raftServer.StartRaft()

	<-signals
}

////////////////////////////////////////
//
// RPCs. These are the actual RPC handlers
//
////////////////////////////////////////

func (raftServer *RaftServer) Discover(ctx context.Context, req *DiscoverRequest) (*DiscoverResponse, error) {
	peer, _ := peer.FromContext(ctx)
	addrPeer, _, _ := net.SplitHostPort(peer.Addr.String())

	// Just move on if we got our own request
	if raftServer.localAddress == addrPeer {
		return &DiscoverResponse{
			Members: raftServer.peerAddresses,
			Caller:  addrPeer,
		}, nil
	}

	// If we got a request from any other node, we sync state
	for _, member := range req.Members {
		raftServer.addPeer(member)
	}

	return &DiscoverResponse{
		Members: raftServer.peerAddresses,
		Caller:  addrPeer,
	}, nil
}

func (raftServer *RaftServer) RequestVote(ctx context.Context, req *RequestVoteRequest) (*RequestVoteResponse, error) {
	// Reply false if term < currentTerm
	if req.Term < raftServer.currentTerm {
		log.Printf("Rejecting vote request from %v due to term mismatch", req.CandidateId)

		return &RequestVoteResponse{
			Term:        raftServer.currentTerm,
			VoteGranted: false,
		}, nil
	}

	// if req.Term > raftServer.currentTerm {
	// 	raftServer.currentTerm = req.Term
	// 	raftServer.state = FOLLOWER

	// }

	// If votedFor is null or candidateId, and candidate’s log is at least as up-to-date as receiver’s log, grant vote
	if raftServer.votedFor == "" || raftServer.votedFor == req.CandidateId {
		if req.LastLogTerm >= raftServer.currentTerm {
			log.Printf("Granting vote request from %v", req.CandidateId)

			raftServer.votedFor = req.CandidateId

			return &RequestVoteResponse{
				Term:        raftServer.currentTerm,
				VoteGranted: true,
			}, nil
		}

		log.Printf("Rejecting vote request from %v due to log mismatch", req.CandidateId)
	} else {
		log.Printf("Rejecting vote request from %v due to already voting for %v", req.CandidateId, raftServer.votedFor)
	}

	return &RequestVoteResponse{
		Term:        raftServer.currentTerm,
		VoteGranted: false,
	}, nil
}

func (raftServer *RaftServer) AppendEntries(ctx context.Context, req *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	raftServer.heartbeat <- true

	// Reply false if term < currentTerm
	if req.Term < raftServer.currentTerm {
		return &AppendEntriesResponse{
			Term:    raftServer.currentTerm,
			Success: false,
		}, nil
	}

	raftServer.state = FOLLOWER

	return &AppendEntriesResponse{
		Term:    raftServer.currentTerm,
		Success: true,
	}, nil
}
