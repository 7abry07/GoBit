package bencode

import (
	"strconv"
	"strings"
)

const MaxDepth = 100

func Decode(input string) (BNode, error) {
	depth := 0
	value, err := decode(&input, &depth)
	if err != nil {
		return BNode{}, err
	}
	if len(input) != 0 {
		return BNode{}, Trailing_input_err
	}
	return value, nil
}

func decode(input *string, depth *int) (BNode, error) {
	*depth++

	if *depth == MaxDepth {
		return BNode{}, Maximum_nesting_err
	}
	if len(*input) == 0 {
		return BNode{}, Empty_input_err
	}

	c := (*input)[0]
	switch c {
	case '0':
		fallthrough
	case '1':
		fallthrough
	case '2':
		fallthrough
	case '3':
		fallthrough
	case '4':
		fallthrough
	case '5':
		fallthrough
	case '6':
		fallthrough
	case '7':
		fallthrough
	case '8':
		fallthrough
	case '9':
		{
			val, err := decodeStr(input)
			*depth--
			if err != nil {
				return BNode{}, err
			}
			return NewStr(val), nil
		}
	case 'i':
		{
			val, err := decodeInt(input)
			*depth--
			if err != nil {
				return BNode{}, err
			}
			return NewInt(val), nil
		}
	case 'l':
		{
			val, err := decodeList(input, depth)
			*depth--
			if err != nil {
				return BNode{}, err
			}
			return NewList(val), nil
		}
	case 'd':
		{
			val, err := decodeDict(input, depth)
			*depth--
			if err != nil {
				return BNode{}, err
			}
			return NewDict(val), nil
		}
	}
	return BNode{}, Invalid_type_err
}

func decodeStr(input *string) (BStr, error) {
	lenEnd := strings.IndexByte(*input, ':')
	if lenEnd == -1 {
		return "", Missing_colon_err
	}

	lenStr := (*input)[0:lenEnd]
	lenInt, err := strconv.Atoi(lenStr)
	if err != nil {
		return "", Invalid_str_length_err
	}
	*input = (*input)[lenEnd+1:]
	if len(*input) < lenInt {
		return "", Length_mismatch_err
	}

	payload := (*input)[0:lenInt]
	*input = (*input)[lenInt:]
	return BStr(payload), nil
}

func decodeInt(input *string) (BInt, error) {
	*input = (*input)[1:]
	intEnd := strings.IndexByte(*input, 'e')
	if intEnd == -1 {
		return 0, Missing_int_term_err
	}
	strVal := (*input)[0:intEnd]
	val, err := strconv.Atoi(strVal)
	if err != nil {
		return 0, Invalid_int_err
	}

	*input = (*input)[intEnd+1:]
	return BInt(val), nil
}

func decodeList(input *string, depth *int) (BList, error) {
	*input = (*input)[1:]
	var list BList
	for {
		if len(*input) == 0 {
			return BList{}, Missing_list_term_err
		}
		if (*input)[0] == 'e' {
			*input = (*input)[1:]
			return list, nil
		}

		val, err := decode(input, depth)
		if err != nil {
			return BList{}, err
		}

		list = append(list, val)
	}
}

func decodeDict(input *string, depth *int) (BDict, error) {
	*input = (*input)[1:]
	node := NewEmptyDict()
	dict, _ := node.Dict()

	previousKey := ""
	first := true

	for {
		if len(*input) == 0 {
			return BDict{}, Missing_dict_term_err
		}
		if (*input)[0] == 'e' {
			*input = (*input)[1:]
			return dict, nil
		}

		keyNode, err := decode(input, depth)
		if err != nil {
			return BDict{}, err
		}

		key, ok := keyNode.Str()
		if !ok {
			return BDict{}, Non_str_key_err
		}
		if key < BStr(previousKey) && !first {
			return BDict{}, Non_sorted_keys_err
		}

		val, err := decode(input, depth)
		if err != nil {
			return BDict{}, err
		}

		_, exists := dict[string(key)]
		if exists {
			return BDict{}, Duplicate_key_err
		}
		dict[string(key)] = val
		previousKey = string(key)
		first = false
	}
}

//
//
//
//
//
//
//
//
//
//
//
//
//
