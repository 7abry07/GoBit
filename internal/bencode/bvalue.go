package bencode

type nodeKind int

const (
	Int_t nodeKind = iota
	Str_t
	List_t
	Dict_t
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
		kind: Int_t,
		_int: input,
	}
}

func NewStr(input BStr) BNode {
	return BNode{
		kind: Str_t,
		_str: input,
	}
}

func NewList(input BList) BNode {
	return BNode{
		kind:  List_t,
		_list: input,
	}
}

func NewEmptyList() BNode {
	return BNode{
		kind:  List_t,
		_list: BList{},
	}
}

func NewDict(input BDict) BNode {
	return BNode{
		kind:  Dict_t,
		_dict: input,
	}
}

func NewEmptyDict() BNode {
	return BNode{
		kind:  Dict_t,
		_dict: make(BDict),
	}
}

func (n BNode) Type() nodeKind {
	return n.kind
}

func (n BNode) Str() (BStr, bool) {
	if n.kind == Str_t {
		return n._str, true
	}
	return "", false
}
func (n BNode) Int() (BInt, bool) {
	if n.kind == Int_t {
		return n._int, true
	}
	return 0, false
}
func (n BNode) List() (BList, bool) {
	if n.kind == List_t {
		return n._list, true
	}
	return []BNode{}, false
}
func (n BNode) Dict() (BDict, bool) {
	if n.kind == Dict_t {
		return n._dict, true
	}
	return map[string]BNode{}, false
}

func (d BDict) Find(k string) (BNode, bool) {
	node, exists := d[k]
	if !exists {
		return BNode{}, false
	}
	return node, true
}

func (d BDict) FindIntOrDef(k string, def BInt) (BInt, bool) {
	node, exists := d[k]
	if !exists {
		return def, false
	}

	value, ok := node.Int()
	if !ok {
		return def, false
	}

	return value, true
}

func (d BDict) FindInt(k string) (BInt, bool) {
	node, exists := d[k]
	if !exists {
		return 0, false
	}

	value, ok := node.Int()
	if !ok {
		return 0, false
	}

	return value, true
}

func (d BDict) FindStrOrDef(k string, def BStr) (BStr, bool) {
	node, exists := d[k]
	if !exists {
		return def, false
	}

	value, ok := node.Str()
	if !ok {
		return def, false
	}

	return value, true
}

func (d BDict) FindStr(k string) (BStr, bool) {
	node, exists := d[k]
	if !exists {
		return "", false
	}

	value, ok := node.Str()
	if !ok {
		return "", false
	}

	return value, true
}

func (d BDict) FindList(k string) (BList, bool) {
	node, exists := d[k]
	if !exists {
		return BList{}, false
	}

	value, ok := node.List()
	if !ok {
		return BList{}, false
	}

	return value, true
}

func (d BDict) FindDict(k string) (BDict, bool) {
	node, exists := d[k]
	if !exists {
		return BDict{}, false
	}

	value, ok := node.Dict()
	if !ok {
		return BDict{}, false
	}

	return value, true
}
