package main

import (
	"GoBit/internal/bencode"
	"fmt"
)

func main() {
	fmt.Println(bencode.Encode("hello"))
	fmt.Println(bencode.Decode("hello"))
}
