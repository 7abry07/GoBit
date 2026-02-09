package bencode

import (
	"fmt"
	"strings"
)

func Encode(n BNode) string {
	var result strings.Builder
	if n.IsInt() {
		result.WriteString(encodeInt(n._int))
	} else if n.IsStr() {
		result.WriteString(encodeStr(n._str))
	} else if n.IsList() {
		result.WriteString(encodeList(n._list))
	} else if n.IsDict() {
		result.WriteString((encodeDict(n._dict)))
	}
	return result.String()
}

func encodeInt(i BInt) string {
	return fmt.Sprintf("i%de", i)
}

func encodeStr(s BStr) string {
	return fmt.Sprintf("%d:%s", len(s), s)
}

func encodeList(l BList) string {
	var result strings.Builder
	result.WriteRune('l')
	for _, val := range l {
		result.WriteString(Encode(val))
	}
	result.WriteRune('e')
	return result.String()
}

func encodeDict(d BDict) string {
	var result strings.Builder
	result.WriteRune('d')
	for k, v := range d {
		result.WriteString(encodeStr(BStr(k)))
		result.WriteString(Encode(v))
	}
	result.WriteRune('e')
	return result.String()
}
