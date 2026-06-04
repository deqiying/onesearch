package main

import (
	"os"

	"github.com/deqiying/onesearch/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
