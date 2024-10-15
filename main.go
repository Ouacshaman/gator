package main

import (
	"fmt"
	"gator/internal/config"
)

func main() {
	cfg, err := config.Read()
	if err != nil{
		fmt.Println("Error: ",err)
		return
	}
	err = cfg.SetUser("Shi")
	if err != nil{
		fmt.Println("Error: ",err)
		return
	}
	fnl, err := config.Read()
	if err != nil{
		fmt.Println("Error: ",err)
		return
	}
	fmt.Printf("DB URL: %s, Current User: %s", fnl.Db_url, fnl.Current_user_name)
}
