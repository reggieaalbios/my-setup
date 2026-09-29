package main

import (
	"fmt"
	"os"

	"github.com/reggieaalbios/my-setup/internal/app"
)

var version = "dev"

func main() {
	if err := app.NewCommand(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
