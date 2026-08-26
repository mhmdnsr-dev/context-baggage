package main

import (
	"fmt"
	"os"

	"github.com/mhmdnsr-dev/context-baggage/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
