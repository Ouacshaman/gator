package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"gator/internal/config"
	"gator/internal/database"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	db     *database.Queries
	config *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
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
	if err != nil {
		fmt.Println(err)
		return
	}
	dbQueries := database.New(db)
	cfg := state{}
	temp_cfg, err := config.Read()
	if err != nil {
		fmt.Println("Errors: ", err)
		return
	}
	cfg.config = &temp_cfg
	cfg.db = dbQueries
	cmds := commands{}
	cmds.handlers = make(map[string]func(*state, command) error)
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
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

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return errors.New("empty arguments")
	}
	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err != nil {
		fmt.Println("Username DNE")
		os.Exit(1)
	}
	err = s.config.SetUser(cmd.args[0])
	if err != nil {
		return err
	}
	fmt.Println("Username have been set to: ", cmd.args[0])
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("no name arguments")
	}
	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err == nil {
		fmt.Println("Username Exist")
		os.Exit(1)
	} else if err != sql.ErrNoRows {
		fmt.Println("Error checking user:", err)
		return err
	}
	user, err := s.db.CreateUser(context.Background(),
		database.CreateUserParams{ID: uuid.New(), CreatedAt: time.Now(),
			UpdatedAt: time.Now(), Name: cmd.args[0]})
	if err != nil {
		fmt.Println("Error creating user:", err)
		os.Exit(1)
	}
	err = s.config.SetUser(cmd.args[0])
	if err != nil {
		return err
	}
	fmt.Printf("User: %s have been created", cmd.args[0])
	fmt.Println(user)
	return nil
}

func handlerReset(s *state, cmd command) error {
	s.db.DeleteAllUsers(context.Background())
	return nil
}

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}
	for _, user := range users {
		if s.config.Current_user_name == user.Name {
			fmt.Println(user.Name + " (current)")
			continue
		}
		fmt.Println(user.Name)
	}
	return nil
}

func handlerAgg(s *state, cmd command) error {
	time_between_reqs := cmd.args[0]
	time_interpret, err := time.ParseDuration(time_between_reqs)
	if err != nil {
		return err
	}
	fmt.Println("Collecting feeds every", time_interpret)
	ticker := time.NewTicker(time_interpret)
	for ; ; <-ticker.C {
		err = scrapeFeeds(s)
		if err != nil {
			fmt.Printf("Error scraping feeds: %v\n", err)
		}
	}
	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	feed, err := s.db.CreateFeed(context.Background(),
		database.CreateFeedParams{ID: uuid.New(), CreatedAt: time.Now(),
			UpdatedAt: time.Now(), Name: cmd.args[0], Url: cmd.args[1],
			UserID: user.ID})
	if err != nil {
		return err
	}
	fmt.Println(feed)
	feedFollow, err := s.db.CreateFeedFollow(context.Background(),
		database.CreateFeedFollowParams{ID: uuid.New(), CreatedAt: time.Now(),
			UpdatedAt: time.Now(), FeedID: feed.ID, UserID: user.ID})
	if err != nil {
		return err
	}
	fmt.Println(feedFollow)
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeed(context.Background())
	if err != nil {
		return err
	}
	for _, v := range feeds {
		fmt.Println(v)
	}
	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	feed, err := s.db.GetFeedByURL(context.Background(), cmd.args[0])
	if err != nil {
		return err
	}
	cff, err := s.db.CreateFeedFollow(context.Background(),
		database.CreateFeedFollowParams{ID: uuid.New(), CreatedAt: time.Now(),
			UpdatedAt: time.Now(), FeedID: feed.ID, UserID: user.ID})
	if err != nil {
		return err
	}
	fmt.Println(cff.FeedName, cff.UserName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	feeds, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}
	for _, v := range feeds {
		fmt.Println(v.FeedName)
	}
	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	err := s.db.DeleteFollow(context.Background(), database.DeleteFollowParams{
		UserID: user.ID, Url: cmd.args[0]})
	if err != nil {
		return err
	}
	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	if len(cmd.args) > 0 {
		limit, _ = strconv.Atoi(cmd.args[0])
	}
	posts, err := s.db.GetPostsUser(context.Background(),
		database.GetPostsUserParams{UserID: user.ID, Limit: int32(limit)})
	if err != nil {
		return err
	}
	for _, v := range posts {
		fmt.Println(v)
	}
	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.config.Current_user_name)
		if err != nil {
			return err
		}
		handler(s, cmd, user)
		return nil
	}
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	res := &RSSFeed{}
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return &RSSFeed{}, err
	}
	req.Header.Add("User-Agent", "gator")
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &RSSFeed{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &RSSFeed{}, err
	}
	if err = xml.Unmarshal(body, &res); err != nil {
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

func scrapeFeeds(s *state) error {
	feed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}
	err = s.db.MarkFeedFetched(context.Background(), feed.ID)
	if err != nil {
		return err
	}
	rssfeed, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		return err
	}
	for _, v := range rssfeed.Channel.Item {
		now := time.Now()
		tm := parseDate(v.PubDate)
		_, err := s.db.CreatePost(context.Background(), database.CreatePostParams{ID: uuid.New(),
			CreatedAt: now, UpdatedAt: now, Title: v.Title, Url: v.Link,
			Description: sql.NullString{String: v.Description, Valid: v.Description != ""},
			PublishedAt: sql.NullTime{Time: tm, Valid: tm != time.Time{}}, FeedID: feed.ID})
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			}
			log.Printf("Error creating post: %v", err)
			continue
		}
	}
	return nil
}

func parseDate(dateStr string) time.Time {
	layouts := []string{
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z",
		"Mon Jan _2 15:04:05 MST 2006",
		"02 Jan 06 15:04 MST",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t
		}
	}
	return time.Now()
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	opFunc, ok := c.handlers[cmd.name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return opFunc(s, cmd)
}
