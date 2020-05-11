package main

import (
	"encoding/xml"
	"fmt"
	"os"

	"github.com/ucarion/c14n"
)

func main() {
	decoder := xml.NewDecoder(os.Stdin)
	canonical, err := c14n.Canonicalize("root", decoder)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(canonical))
}
