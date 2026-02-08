package bencode

import "fmt"

func Encode(input string) string {
	return fmt.Sprintf("encoded <%v>", input)
}
