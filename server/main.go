package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"

	api "github.com/wbyatt/rolodex/api"
)

type server struct {
	api.RolodexServer
	store Store
}

func (s *server) Set(ctx context.Context, req *api.SetRequest) (*api.SetResponse, error) {
	value, err := s.store.Set(req.Key, req.Value)

	if err != nil {
		return nil, err
	}

	return &api.SetResponse{
		Value: value,
	}, nil
}

func (s *server) Get(ctx context.Context, req *api.GetRequest) (*api.GetResponse, error) {
	value, err := s.store.Get(req.Key)

	if err != nil {
		return nil, err
	}

	return &api.GetResponse{
		Value: value,
	}, nil
}

func (s *server) Delete(ctx context.Context, req *api.DeleteRequest) (*api.DeleteResponse, error) {
	value, err := s.store.Delete(req.Key)

	if err != nil {
		return nil, err
	}

	return &api.DeleteResponse{Value: value}, nil
}

func (s *server) List(ctx context.Context, _empty_req *emptypb.Empty) (*api.ListResponse, error) {
	listPromise := s.store.AsyncList()

	response := &api.ListResponse{}

	for pair := range listPromise {
		key :=
			&api.KeyValuePair{
				Key:   pair.key,
				Value: pair.value,
			}

		response.Keys = append(response.Keys, key)
	}

	return response, nil
}

func Main(port *int) {
	listener, error := net.Listen("tcp", fmt.Sprintf(":%d", *port))

	if error != nil {
		panic(error)
	}

	grpcServer := grpc.NewServer()
	api.RegisterRolodexServer(grpcServer, &server{store: *NewStore()})
	reflection.Register(grpcServer)

	log.Printf("Starting server at %v", listener.Addr())

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
