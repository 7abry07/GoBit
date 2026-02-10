package bencode

import (
	"fmt"
	"sort"
	"strings"
)

func Encode(n BNode) string {
	var result strings.Builder
	switch n.kind {

	case Int_t:
		result.WriteString(encodeInt(n._int))
	case Str_t:
		result.WriteString(encodeStr(n._str))
	case List_t:
		result.WriteString(encodeList(n._list))
	case Dict_t:
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

	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		result.WriteString(encodeStr(BStr(k)))
		result.WriteString(Encode(d[k]))
	}
	result.WriteRune('e')
	return result.String()
}
