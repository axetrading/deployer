package main

import (
	"context"
	"log"
	"os"

	"github.com/axetrading/deployer/internal/deployer"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("usage: deployer JSON_PARAMS")
	}
	inputs, err := deployer.GetInputs(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}
	if err := deployer.Main(context.Background(), inputs); err != nil {
		log.Fatalln(err)
	}
}
