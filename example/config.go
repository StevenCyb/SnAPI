package api

import (
	"fmt"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

type Config struct {
	Name string `arg:"name,env=NAME,default=SnAPI"`
}

var config Config

// @snapi.setup
func LoadConfig() error {
	fmt.Println("Loading configuration...")
	conf, err := runtime.LoadConfig[Config]()
	if err != nil {
		return err
	}
	config = conf
	return nil
}
