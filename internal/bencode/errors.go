package bencode

import "errors"

var (
	empty_input_err        = errors.New("")
	invalid_type_err       = errors.New("invalid type specifier encountered")
	maximum_nesting_err    = errors.New("maximum nesting limit excedeed")
	trailing_input_err     = errors.New("trailing input not allowed")
	invalid_int_err        = errors.New("invalid integer encountered")
	missing_int_term_err   = errors.New("integer terminator not found")
	invalid_str_length_err = errors.New("the string length is invalid")
	length_mismatch_err    = errors.New("the length of the string doesn't match the payload")
	missing_colon_err      = errors.New("the colon between length and payload is missing")
	missing_list_term_err  = errors.New("list terminator not found")
	missing_dict_term_err  = errors.New("dictionary terminator not found")
	non_str_key_err        = errors.New("key to dictionary value not a string")
	duplicate_key_err      = errors.New("key to dictionary value already exists")
	non_sorted_keys_err    = errors.New("keys in the dictionary aren't sorted lexicographically")
)
