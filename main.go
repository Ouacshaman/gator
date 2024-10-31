package main

import (
	"fmt"
	"os"
	"html"
	"io"
	"gator/internal/config"
	"errors"
	"encoding/xml"
	"database/sql"
	"context"
	"github.com/google/uuid"
	"time"
	"net/http"
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

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
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
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", handlerAddFeed)
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

func handlerUsers(s *state, cmd command) error{
	users, err := s.db.GetUsers(context.Background())
	if err != nil{
		return err
	}
	for _, user := range users{
		if s.config.Current_user_name == user.Name{
			fmt.Println(user.Name+" (current)")
			continue
		}
		fmt.Println(user.Name)
	}
	return nil
}

func handlerAgg(s *state, cmd command) error{
	res, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil{
		return err
	}
	fmt.Println(res)
	return nil
}

func handlerAddFeed(s *state, cmd command) error{
	curr := s.config.Current_user_name
	usr, err := s.db.GetUser(context.Background(), curr)
	if err != nil{
		return err
	}
	feed, err := s.db.CreateFeed(context.Background(),
		database.CreateFeedParams{ID: uuid.New(), CreatedAt: time.Now(),
			UpdatedAt: time.Now(), Name: cmd.args[0], Url: cmd.args[1], 
			UserID: usr.ID})
	if err != nil{
		return err
	}
	fmt.Println(feed)
	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error){
	res := &RSSFeed{}
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil{
		return &RSSFeed{}, err
	}
	req.Header.Add("User-Agent","gator")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil{
		return &RSSFeed{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil{
		return &RSSFeed{}, err
	}
	if err = xml.Unmarshal(body, &res); err != nil{
		return &RSSFeed{}, err
	}
	res.Channel.Title = html.UnescapeString(res.Channel.Title)
	res.Channel.Description = html.UnescapeString(res.Channel.Description)
	for i := range res.Channel.Item {
		res.Channel.Item[i].Title = html.UnescapeString(res.Channel.Item[i].Title)
		res.Channel.Item[i].Description = html.UnescapeString(res.Channel.Item[i].Description)
	}
	return res, nil
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
