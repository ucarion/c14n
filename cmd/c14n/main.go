package main

import (
	"encoding/xml"
	"fmt"
	"os"

	"github.com/ucarion/c14n"
)

func main() {
	decoder := xml.NewDecoder(os.Stdin)
	out, err := c14n.Canonicalize(decoder)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Println(string(out))
}
