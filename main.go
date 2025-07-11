package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gliderlabs/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"github.com/sector-f/ssh-cli-forum/sqlite"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	defer logger.Sync()

	service, err := sqlite.Open(logger, "./forum.db")
	if err != nil {
		log.Fatal(err)
	}

	s := &ssh.Server{
		Addr:             "127.0.0.1:2222",
		Handler:          sessionHandler,
		PublicKeyHandler: authHandler(service),
	}
	defer s.Close()

	log.Fatal(s.ListenAndServe())
}

func sessionHandler(s ssh.Session) {

}

func authHandler(service forum.Service) func(ctx ssh.Context, key ssh.PublicKey) bool {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		user, err := service.GetUserByUsername(ctx.User())
		if err != nil {
			// TODO: add logging
			return false
		}

		// ssh.KeysEqual is the canonical way to compare public keys.
		return ssh.KeysEqual(user.PubKey, key)
	}
}
