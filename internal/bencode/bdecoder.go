package bencode

import (
	"fmt"
)

func Decode(input string) string {
	return fmt.Sprintf("decoded <%v>", input)
}

