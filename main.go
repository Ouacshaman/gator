package main

import (
	"fmt"
	"os"
	"gator/internal/config"
	"errors"
)

type state struct{
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
	cfg := state{}
	var err error
	temp_cfg, err := config.Read()
	if err != nil{
		fmt.Println("Errors: ", err)
		return
	}
	cfg.config = &temp_cfg
	cmds := commands{}
	cmds.handlers = make(map[string]func(*state, command) error)
	cmds.register("login",handlerLogin)
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
	err := s.config.SetUser(cmd.args[0])
	if err != nil{
		return err
	}
	fmt.Println("Username have been set to: ", cmd.args[0])
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
