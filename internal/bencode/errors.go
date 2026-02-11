package bencode

import "errors"

var (
	Empty_input_err        = errors.New("the input is empty")
	Invalid_type_err       = errors.New("invalid type specifier encountered")
	Maximum_nesting_err    = errors.New("maximum nesting limit excedeed")
	Trailing_input_err     = errors.New("trailing input not allowed")
	Invalid_int_err        = errors.New("invalid integer encountered")
	Missing_int_term_err   = errors.New("integer terminator not found")
	Invalid_str_length_err = errors.New("the string length is invalid")
	Length_mismatch_err    = errors.New("the length of the string doesn't match the payload")
	Missing_colon_err      = errors.New("the colon between length and payload is missing")
	Missing_list_term_err  = errors.New("list terminator not found")
	Missing_dict_term_err  = errors.New("dictionary terminator not found")
	Non_str_key_err        = errors.New("key to dictionary value not a string")
	Duplicate_key_err      = errors.New("key to dictionary value already exists")
	Non_sorted_keys_err    = errors.New("keys in the dictionary aren't sorted lexicographically")
)
