package main

import (
	"fmt"
	"os"
	"gator/internal/config"
	"errors"
	"database/sql"
	"context"
	"github.com/google/uuid"
	"time"
	"gator/internal/database"
	_ "github.com/lib/pq"
)

type state struct{
	db  *database.Queries
	config *config.Config
}

type command struct{
	name string
	args  []string
}

type commands struct{
	handlers map[string]func(*state, command) error
}

func main() {
	db, err := sql.Open("postgres", "postgres://shihong:@localhost:5432/gator?sslmode=disable")
	if err != nil{
		fmt.Println(err)
		return
	}
	dbQueries := database.New(db)
	cfg := state{}
	temp_cfg, err := config.Read()
	if err != nil{
		fmt.Println("Errors: ", err)
		return
	}
	cfg.config = &temp_cfg
	cfg.db = dbQueries
	cmds := commands{}
	cmds.handlers = make(map[string]func(*state, command) error)
	cmds.register("login",handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	args := os.Args
	if len(args) < 2 {
		fmt.Println("not enough args")
		os.Exit(1)
	}
	cmd := command{
		name: args[1],
		args: args[2:],
	}
	err = cmds.run(&cfg, cmd)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func handlerLogin(s *state, cmd command) error{
	if len(cmd.args) == 0 {
		return errors.New("empty arguments")
	}
	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err != nil{
		fmt.Println("Username DNE")
		os.Exit(1)
	}
	err = s.config.SetUser(cmd.args[0])
	if err != nil{
		return err
	}
	fmt.Println("Username have been set to: ", cmd.args[0])
	return nil
}

func handlerRegister(s *state, cmd command) error{
	if len(cmd.args) < 1 {
		return errors.New("no name arguments")
	}
	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err == nil{
		fmt.Println("Username Exist")
		os.Exit(1)
	} else if err != sql.ErrNoRows {
		fmt.Println("Error checking user:", err)
		return err
	}
	user, err := s.db.CreateUser(context.Background(),
		database.CreateUserParams{ID: uuid.New(),CreatedAt: time.Now(),
					UpdatedAt: time.Now(), Name: cmd.args[0]})
	if err != nil{
		fmt.Println("Error creating user:", err)
		os.Exit(1)
	}
	err = s.config.SetUser(cmd.args[0])
	if err != nil{
		return err
	}
	fmt.Printf("User: %s have been created", cmd.args[0])
	fmt.Println(user)
	return nil
}

func handlerReset(s *state, cmd command) error{
	s.db.DeleteAllUsers(context.Background())
	return nil
}

func (c *commands) register(name string, f func(*state, command) error){
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error{
	opFunc, ok := c.handlers[cmd.name]
	if !ok{
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return opFunc(s,cmd)
}
