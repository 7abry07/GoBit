package bencode

import (
	"errors"
)

type nodeKind int

const (
	IntType nodeKind = iota
	StrType
	ListType
	DictType
)

type BInt int
type BStr string
type BList []BNode
type BDict map[string]BNode

type BNode struct {
	kind  nodeKind
	_int  BInt
	_str  BStr
	_list BList
	_dict BDict
}

func NewInt(input BInt) BNode {
	return BNode{
		kind: IntType,
		_int: input,
	}
}

func NewStr(input BStr) BNode {
	return BNode{
		kind: StrType,
		_str: input,
	}
}

func NewList(input BList) BNode {
	return BNode{
		kind:  ListType,
		_list: input,
	}
}

func NewEmptyList() BNode {
	return BNode{
		kind:  ListType,
		_list: BList{},
	}
}

func NewDict(input BDict) BNode {
	return BNode{
		kind:  DictType,
		_dict: input,
	}
}

func NewEmptyDict() BNode {
	return BNode{
		kind:  DictType,
		_dict: make(BDict),
	}
}

func (n BNode) IsInt() bool {
	return n.kind == IntType
}
func (n BNode) IsStr() bool {
	return n.kind == StrType
}
func (n BNode) IsList() bool {
	return n.kind == ListType
}
func (n BNode) IsDict() bool {
	return n.kind == DictType
}

func (n BNode) Str() (BStr, error) {
	if n.kind == StrType {
		return n._str, nil
	}
	return "", errors.New("not string")
}
func (n BNode) Int() (BInt, error) {
	if n.kind == IntType {
		return n._int, nil
	}
	return 0, errors.New("not int")
}
func (n BNode) List() (BList, error) {
	if n.kind == ListType {
		return n._list, nil
	}
	return []BNode{}, errors.New("not list")
}
func (n BNode) Dict() (BDict, error) {
	if n.kind == DictType {
		return n._dict, nil
	}
	return map[string]BNode{}, errors.New("not dict")
}

func (d BDict) FindIntOrDef(k string, def BInt) (BInt, bool) {
	node, exists := d[k]
	if exists == false {
		return def, false
	}

	value, valueErr := node.Int()
	if valueErr != nil {
		return def, false
	}

	return value, true
}

func (d BDict) FindInt(k string) (BInt, bool) {
	node, exists := d[k]
	if exists == false {
		return 0, false
	}

	value, valueErr := node.Int()
	if valueErr != nil {
		return 0, false
	}

	return value, true
}

func (d BDict) FindStrOrDef(k string, def BStr) (BStr, bool) {
	node, exists := d[k]
	if exists == false {
		return def, false
	}

	value, valueErr := node.Str()
	if valueErr != nil {
		return def, false
	}

	return value, true
}

func (d BDict) FindStr(k string) (BStr, bool) {
	node, exists := d[k]
	if exists == false {
		return "", false
	}

	value, valueErr := node.Str()
	if valueErr != nil {
		return "", false
	}

	return value, true
}

func (d BDict) FindList(k string) (BList, bool) {
	node, exists := d[k]
	if exists == false {
		return BList{}, false
	}

	value, valueErr := node.List()
	if valueErr != nil {
		return BList{}, false
	}

	return value, true
}

func (d BDict) FindDict(k string) (BDict, bool) {
	node, exists := d[k]
	if exists == false {
		return BDict{}, false
	}

	value, valueErr := node.Dict()
	if valueErr != nil {
		return BDict{}, false
	}

	return value, true
}
