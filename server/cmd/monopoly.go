package main

import (
	"context"
	"fmt"
	flag "go.badoo.dev/core/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	monopoly "kek/server"
	"log"
	"net"
)

type state uint32
const (
	Idle state = iota
	Rolling
	Trading
)

type user struct {
	pubId uint32
	name string
	st state
	trading string
	streams []monopoly.Monopoly_SubsServer
}

type game struct {
	users map[string]*user
	//chat []string
	locs map[uint32]uint32
}

type MyMonopolyServer struct {
	games map[string]*game
}

func (m *MyMonopolyServer) Chat(ctx context.Context, req *monopoly.ChatRequest) (*monopoly.ChatResponse, error) {
	var userId, gameId string
	var err error
	if userId, gameId, err = authorize(ctx); err != nil {
		return nil, err
	}
	game, ok := m.games[gameId]
	if !ok {
		return nil, fmt.Errorf("no such game")
	}
	user, ok := game.users[userId]
	if !ok {
		return nil, fmt.Errorf("no such user")
	}
	req.GetLine()

	resp := &monopoly.SubsRespStream{Chat: []string{user.name + ": " + req.GetLine()}}

	for _, u := range game.users {
		for _, s := range u.streams {
			err := s.Send(resp)
			log.Println("send chat", err)
		}
	}

	return &monopoly.ChatResponse{}, nil
}

func authorize(ctx context.Context) (userId, gameId string, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		err = fmt.Errorf("no md")
		return
	}
	userIds := md.Get("user_id")
	if len(userIds) != 1 {
		err = fmt.Errorf("user_id wrong len")
		return
	}
	userId = userIds[0]

	gameIds := md.Get("game_id")
	if len(gameIds) != 1 {
		err = fmt.Errorf("game_id wrong len")
		return
	}
	gameId = gameIds[0]
	log.Println("user", userId, "game", gameId)
	return
}

func (m *MyMonopolyServer) Subs(req *monopoly.SubsRequest, oStr monopoly.Monopoly_SubsServer) error {
	var userId, gameId string
	var err error
	ctx := oStr.Context()

	log.Println("Subs ctx", ctx)

	if p, ok := peer.FromContext(ctx); ok {
		log.Println("peerKey", p)
	}

	sts := grpc.ServerTransportStreamFromContext(ctx)
	log.Println(sts.Method())

	if userId, gameId, err = authorize(ctx); err != nil {
		return err
	}

	game, ok := m.games[gameId]
	if !ok {
		return fmt.Errorf("no such game")
	}
	user, ok := game.users[userId]
	if !ok {
		return fmt.Errorf("no such user")
	}


	user.streams = append(user.streams, oStr)

	log.Println("Subs streaming call: waiting ctx.Done")
	<- ctx.Done()
	log.Println("Subs streaming call: waited ctx.Done")

	return nil
}

func (m *MyMonopolyServer) RollDice(context.Context, *monopoly.RollDiceRequest) (*monopoly.RollDiceResponse, error) {
	panic("implement me")
}



func main() {
	s := grpc.NewServer()

	m := &MyMonopolyServer{}
	m.games = make(map[string]*game)
	m.games["123"] = &game{locs:  nil, users: make(map[string]*user)}
	m.games["123"].users["456"] = &user{
		pubId:   1488,
		name:    "Kek",
		st:      0,
		trading: "",
		streams: make([]monopoly.Monopoly_SubsServer, 0),
	}

	monopoly.RegisterMonopolyServer(s, m)

	dir := flag.String("web", "", "web root")
	flag.Parse()


	//go mitm("0.0.0.0:8088", "localhost:8081")

	go jsWebsocketProxy(*dir, "0.0.0.0:8080", "localhost:8081")

	sock, err := net.Listen("tcp", "0.0.0.0:8081")
	if err != nil {
		log.Fatalln("svc listen", err)
	}

	err = s.Serve(sock)
	log.Fatalln("svc serve", err)
}