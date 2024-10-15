package config

import (
	"os"
	"fmt"
	"encoding/json"
	"path/filepath"
)

type Config struct{
	Db_url 		  string `json:"db_url"`
	Current_user_name string `json:"current_user_name"`
}

const configFileName = ".gatorconfig.json"

func Read() (Config, error){
	home_dir, err := os.UserHomeDir()
	if err != nil{
		fmt.Println("Error:", err)
		return Config{}, err
	}
	data, err := os.ReadFile(filepath.Join(home_dir,configFileName))
	if err != nil{
		fmt.Println("Error:", err)
		return Config{}, err
	}
	cfg := Config{}
	err = json.Unmarshal(data, &cfg)
	if err != nil{
		fmt.Println("Error:", err)
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) SetUser(username string) error{
	c.Current_user_name = username;
	dat, err := json.Marshal(c)
	if err != nil{
		return err
	}
	home_dir, err := os.UserHomeDir()
	if err != nil{
		fmt.Println("Error:", err)
		return err
	}
	err = os.WriteFile(filepath.Join(home_dir,configFileName), dat, 0666)
	if err != nil {
		return err
	}
	return nil
}
