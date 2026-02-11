package bencode_test

import (
	"GoBit/internal/bencode"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestSimpleInt(t *testing.T) {
	input := "i56e"
	val, err := bencode.Decode(input)
	if err != nil {
		t.Errorf("expected: [%v] | got: [%v]", 56, err)
	}
	intval, ok := val.Int()
	if !ok {
		t.Errorf("expected: [%v] | got: [%v]", 56, "not int")
	}
	if intval != 56 {
		t.Errorf("expected: [%v] | got: [%v]", 56, intval)
	}
}

func TestSimpleString(t *testing.T) {
	input := "5:hello"
	val, err := bencode.Decode(input)
	if err != nil {
		t.Errorf("expected: [%v] | got: [%v]", "'hello'", err)
	}
	strval, ok := val.Str()
	if !ok {
		t.Errorf("expected: [%v] | got: [%v]", "'hello'", "not string")
	}
	if strval != "hello" {
		t.Errorf("expected: [%v] | got: [%v]", "'hello'", strval)
	}
}

func TestSimpleList(t *testing.T) {
	input := "l5:helloi56ee"
	val, err := bencode.Decode(input)
	if err != nil {
		t.Errorf("expected: [%v] | got: [%v]", "list", err)
	}
	listval, ok := val.List()
	if !ok {
		t.Errorf("expected: [%v] | got: [%v]", "list", "not list")
	}

	stritem, ok := listval[0].Str()
	intitem, ok2 := listval[1].Int()

	if !ok {
		t.Errorf("expected item: %v->[%v] | got: [%v]", 0, "hello", "not string")
	}
	if !ok2 {
		t.Errorf("expected item: %v->[%v] | got: [%v]", 1, 56, "not int")
	}

	if stritem != "hello" {
		t.Errorf("expected item: %v->[%v] | got: [%v]", 0, "hello", stritem)
	}
	if intitem != 56 {
		t.Errorf("expected item: %v->[%v] | got: [%v]", 1, 56, intitem)
	}
}

func TestSimpleDict(t *testing.T) {
	input := "d5:helloi56ee"
	val, err := bencode.Decode(input)
	if err != nil {
		t.Errorf("expected: [%v] | got: [%v]", "dict", err)
	}
	dictval, ok := val.Dict()
	if !ok {
		t.Errorf("expected: [%v] | got: [%v]", "dict", "not dict")
	}

	value, ok := dictval.FindInt("hello")
	if !ok {
		t.Errorf("expected key-value: [%v-%v] | got: [%v]", "hello", 56, "key not found")
	}
	if value != 56 {
		t.Errorf("expected key-value: [%v-%v] | got: [%v-%v]", "hello", 56, "hello", value)
	}
}

func TestEmptyInput(t *testing.T) {
	input := ""
	_, err := bencode.Decode(input)
	if err != bencode.Empty_input_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Empty_input_err, err)
	}
}

func TestMaxDepth(t *testing.T) {
	var input strings.Builder
	for i := range 200 {
		if i < 100 {
			input.WriteByte('l')
		} else {
			input.WriteByte('e')
		}
	}
	_, err := bencode.Decode(input.String())
	if err != bencode.Maximum_nesting_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Maximum_nesting_err, err)
	}
}

func TestInvalidType(t *testing.T) {
	input := "t"
	_, err := bencode.Decode(input)
	if err != bencode.Invalid_type_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Invalid_type_err, err)
	}
}

func TestTrailingInput(t *testing.T) {
	input := "5:hellotrailing"
	_, err := bencode.Decode(input)
	if err != bencode.Trailing_input_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Trailing_input_err, err)
	}
}

func TestInvalidInteger(t *testing.T) {
	input := fmt.Sprintf("i%v0e", math.MaxInt64)
	_, err := bencode.Decode(input)
	if err != bencode.Invalid_int_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Invalid_int_err, err)
	}

	input = fmt.Sprintf("i%v0e", math.MinInt64)
	_, err2 := bencode.Decode(input)
	if err2 != bencode.Invalid_int_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Invalid_int_err, err2)
	}
}

func TestMissingIntTerm(t *testing.T) {
	input := "i65"
	_, err := bencode.Decode(input)
	if err != bencode.Missing_int_term_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Missing_int_term_err, err)
	}
}

func TestInvalidStringLength(t *testing.T) {
	input := "5h3:hello"
	_, err := bencode.Decode(input)
	if err != bencode.Invalid_str_length_err {
		t.Errorf("expected: %v | got: %v", bencode.Invalid_str_length_err, err)
	}
}

func TestLengthMismatch(t *testing.T) {
	input := "5:hell"
	_, err := bencode.Decode(input)
	if err != bencode.Length_mismatch_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Length_mismatch_err, err)
	}
}

func TestMissingColon(t *testing.T) {
	input := "5hello"
	_, err := bencode.Decode(input)
	if err != bencode.Missing_colon_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Missing_colon_err, err)
	}
}

func TestMissingListTerm(t *testing.T) {
	input := "l5:hello"
	_, err := bencode.Decode(input)
	if err != bencode.Missing_list_term_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Missing_list_term_err, err)
	}
}

func TestMissingDictTerm(t *testing.T) {
	input := "d5:helloi56e"
	_, err := bencode.Decode(input)
	if err != bencode.Missing_dict_term_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Missing_dict_term_err, err)
	}
}

func TestNonStrKey(t *testing.T) {
	input := "di56e5:helloe"
	_, err := bencode.Decode(input)
	if err != bencode.Non_str_key_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Non_str_key_err, err)
	}
}

func TestNonSortedKey(t *testing.T) {
	input := "d4:zetai10e5:alphai10ee"
	_, err := bencode.Decode(input)
	if err != bencode.Non_sorted_keys_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Non_sorted_keys_err, err)
	}
}

func TestDuplicateKey(t *testing.T) {
	input := "d5:hello2:hi5:helloi43ee"
	_, err := bencode.Decode(input)
	if err != bencode.Duplicate_key_err {
		t.Errorf("expected: [%v] | got: [%v]", bencode.Duplicate_key_err, err)
	}
}

//
// ENCODER
//

func TestSimpleEncoding(t *testing.T) {
	input := "d4:helli43e5:hello2:hie"
	val, err := bencode.Decode(input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	encoded := bencode.Encode(val)
	if encoded != input {
		t.Errorf("expected: [%v] | got: [%v]", input, encoded)
	}
}
