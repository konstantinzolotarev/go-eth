package abi

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/defiweb/go-eth/hexutil"
	"github.com/defiweb/go-eth/types"

	"github.com/defiweb/go-anymapper"
)

// Value represents a value that can be marshaled to and from ABI.
//
// https://docs.soliditylang.org/en/develop/abi-spec.html#strict-encoding-mode
type Value interface {
	// DynamicType indicates whether the type is dynamic.
	DynamicType() bool

	// EncodeABI returns the ABI encoding of the value.
	EncodeABI() (Words, error)

	// DecodeABI sets the value from the ABI encoding.
	DecodeABI(Words) (int, error)
}

type TupleValue struct {
	elems []TupleValueElem
}

type TupleValueElem struct {
	Value Value
	Name  string
}

func NewTupleValue(elems ...TupleValueElem) *TupleValue {
	return &TupleValue{elems: elems}
}

func (t *TupleValue) Size() int {
	return len(t.elems)
}

func (t *TupleValue) Elements() []TupleValueElem {
	return t.elems
}

func (t *TupleValue) Map() map[string]Value {
	m := make(map[string]Value)
	for _, elem := range t.elems {
		m[elem.Name] = elem.Value
	}
	return m
}

func (t *TupleValue) Elem(idx int) TupleValueElem {
	if idx < 0 || idx >= len(t.elems) {
		return TupleValueElem{}
	}
	return t.elems[idx]
}

func (t *TupleValue) AddElem(elem TupleValueElem) {
	t.elems = append(t.elems, elem)
}

func (t *TupleValue) SetElem(idx int, elem TupleValueElem) error {
	if idx < 0 || idx >= len(t.elems) {
		return fmt.Errorf("abi: index out of range: %d", idx)
	}
	t.elems[idx] = elem
	return nil
}

func (t *TupleValue) DynamicType() bool {
	for _, elem := range t.elems {
		if elem.Value.DynamicType() {
			return true
		}
	}
	return false
}

func (t *TupleValue) EncodeABI() (Words, error) {
	elems := make([]Value, len(t.elems))
	for i, elem := range t.elems {
		elems[i] = elem.Value
	}
	return encodeTuple(elems)
}

func (t *TupleValue) DecodeABI(words Words) (int, error) {
	elems := make([]Value, len(t.elems))
	for i, elem := range t.elems {
		elems[i] = elem.Value
	}
	return decodeTuple(&elems, words)
}

func (t *TupleValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	return m.MapRefl(src, reflect.ValueOf(t.Map()))
}

func (t *TupleValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	return m.MapRefl(reflect.ValueOf(t.Map()), dest)
}

type ArrayValue struct {
	elems []Value
	typ   Type
}

func NewArrayValue(typ Type, elems ...Value) *ArrayValue {
	return &ArrayValue{elems: elems, typ: typ}
}

func (a *ArrayValue) Length() int {
	return len(a.elems)
}

func (a *ArrayValue) Elements() []Value {
	return a.elems
}

func (a *ArrayValue) Type() Type {
	return a.typ
}

func (a *ArrayValue) Elem(idx int) Value {
	if idx < 0 || idx >= len(a.elems) {
		return nil
	}
	return a.elems[idx]
}

func (a *ArrayValue) AddElem(v Value) {
	a.elems = append(a.elems, v)
}

func (a *ArrayValue) SetElem(idx int, v Value) error {
	if idx < 0 || idx >= len(a.elems) {
		return fmt.Errorf("abi: array index out of bounds")
	}
	a.elems[idx] = v
	return nil
}

func (a *ArrayValue) DynamicType() bool {
	return true
}

func (a *ArrayValue) EncodeABI() (Words, error) {
	return encodeArray(a.elems)
}

func (a *ArrayValue) DecodeABI(data Words) (int, error) {
	return decodeArray(&a.elems, data, a.typ)
}

func (a *ArrayValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	if src.Kind() != reflect.Slice && src.Kind() != reflect.Array {
		return fmt.Errorf("abi: cannot map array from %s", src.Kind())
	}
	a.elems = make([]Value, src.Len())
	for i := 0; i < src.Len(); i++ {
		a.elems[i] = a.typ.New()
	}
	return m.MapRefl(src, reflect.ValueOf(&a.elems))
}

func (a *ArrayValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	return m.MapRefl(reflect.ValueOf(&a.elems), dest)
}

type FixedArrayValue struct {
	elems []Value
	typ   Type
}

func NewFixedArrayValue(typ Type, size int) *FixedArrayValue {
	return &FixedArrayValue{elems: make([]Value, size), typ: typ}
}

func (a *FixedArrayValue) Size() int {
	return len(a.elems)
}

func (a *FixedArrayValue) Elements() []Value {
	return a.elems
}

func (a *FixedArrayValue) Type() Type {
	return a.typ
}

func (a *FixedArrayValue) Elem(idx int) Value {
	if idx < 0 || idx >= len(a.elems) {
		return nil
	}
	return a.elems[idx]
}

func (a *FixedArrayValue) SetElem(idx int, v Value) error {
	if idx < 0 || idx >= len(a.elems) {
		return fmt.Errorf("abi: array index out of bounds")
	}
	a.elems[idx] = v
	return nil
}

func (a *FixedArrayValue) DynamicType() bool {
	return false
}

func (a *FixedArrayValue) EncodeABI() (Words, error) {
	return encodeFixedArray(a.elems)
}

func (a *FixedArrayValue) DecodeABI(data Words) (int, error) {
	return decodeFixedArray(&a.elems, data, a.typ, len(a.elems))
}

func (a *FixedArrayValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	if src.Kind() != reflect.Slice && src.Kind() != reflect.Array {
		return fmt.Errorf("abi: cannot map %s to array[%d]", src.Type(), len(a.elems))
	}
	if src.Len() != len(a.elems) {
		return fmt.Errorf("abi: cannot map %d elements to array[%d]", src.Len(), len(a.elems))
	}
	for i := 0; i < len(a.elems); i++ {
		a.elems[i] = a.typ.New()
	}
	return m.MapRefl(src, reflect.ValueOf(&a.elems))
}

func (a *FixedArrayValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	return m.MapRefl(reflect.ValueOf(&a.elems), dest)
}

type BytesValue struct {
	data []byte
}

func NewBytesValue() *BytesValue {
	return &BytesValue{}
}

func (b *BytesValue) Length() int {
	return len(b.data)
}

func (b *BytesValue) Bytes() []byte {
	return b.data
}

func (b *BytesValue) String() string {
	return string(b.data)
}

func (b *BytesValue) Hex() string {
	return hexutil.BytesToHex(b.data)
}

func (b *BytesValue) SetBytes(d []byte) {
	b.data = d
}

func (b *BytesValue) SetString(s string) {
	b.data = []byte(s)
}

func (b *BytesValue) SetHex(s string) error {
	data, err := hexutil.HexToBytes(s)
	if err != nil {
		return err
	}
	b.data = data
	return nil
}

func (b *BytesValue) DynamicType() bool {
	return true
}

func (b *BytesValue) EncodeABI() (Words, error) {
	return encodeBytes(b.data)
}

func (b *BytesValue) DecodeABI(data Words) (int, error) {
	return decodeBytes(&b.data, data)
}

func (b *BytesValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	return m.MapRefl(src, reflect.ValueOf(&b.data))
}

func (b *BytesValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	return m.MapRefl(reflect.ValueOf(&b.data), dest)
}

type StringValue struct {
	data []byte
}

func NewStringValue() *StringValue {
	return &StringValue{}
}

func (s *StringValue) Length() int {
	return len(s.data)
}

func (s *StringValue) Bytes() []byte {
	return s.data
}

func (s *StringValue) String() string {
	return string(s.data)
}

func (s *StringValue) SetBytes(v []byte) {
	s.data = v
}

func (s *StringValue) SetString(v string) {
	s.data = []byte(v)
}

func (s *StringValue) DynamicType() bool {
	return true
}

func (s *StringValue) EncodeABI() (Words, error) {
	return encodeBytes(s.data)
}

func (s *StringValue) DecodeABI(data Words) (int, error) {
	return decodeBytes(&s.data, data)
}

func (s *StringValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	return m.MapRefl(src, reflect.ValueOf(&s.data))
}

func (s *StringValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	return m.MapRefl(reflect.ValueOf(&s.data), dest)
}

type FixedBytesValue struct {
	data []byte
}

func NewFixedBytesValue(size int) *FixedBytesValue {
	return &FixedBytesValue{data: make([]byte, size)}
}

func (b *FixedBytesValue) Size() int {
	return len(b.data)
}

func (b *FixedBytesValue) Bytes() []byte {
	return b.data
}

func (b *FixedBytesValue) String() string {
	return string(b.data)
}

func (b *FixedBytesValue) Hex() string {
	return hexutil.BytesToHex(b.data)
}

func (b *FixedBytesValue) SetBytesPadRight(d []byte) error {
	if len(d) > len(b.data) {
		return fmt.Errorf("abi: cannot set %d bytes into bytes%d", len(d), len(b.data))
	}
	copy(b.data, d)
	for i := len(d); i < len(b.data); i++ {
		b.data[i] = 0
	}
	return nil
}

func (b *FixedBytesValue) SetBytesPadLeft(d []byte) error {
	if len(d) > len(b.data) {
		return fmt.Errorf("abi: cannot set %d bytes into bytes%d", len(d), len(b.data))
	}
	copy(b.data[len(b.data)-len(d):], d)
	for i := 0; i < len(b.data)-len(d); i++ {
		b.data[i] = 0
	}
	return nil
}

func (b *FixedBytesValue) SetString(s string) error {
	return b.SetBytesPadRight([]byte(s))
}

func (b *FixedBytesValue) SetHex(s string) error {
	data, err := hexutil.HexToBytes(s)
	if err != nil {
		return err
	}
	return b.SetBytesPadLeft(data)
}

func (b *FixedBytesValue) DynamicType() bool {
	return false
}

func (b *FixedBytesValue) EncodeABI() (Words, error) {
	if len(b.data) == 0 {
		return nil, fmt.Errorf("abi: invalid size, use NewFixedBytesType() to create a new FixedBytesValue")
	}
	return encodeFixedBytes(b.data)
}

func (b *FixedBytesValue) DecodeABI(data Words) (int, error) {
	if len(b.data) == 0 {
		return 0, fmt.Errorf("abi: invalid size, use NewFixedBytesType() to create a new FixedBytesValue")
	}
	return decodeFixedBytes(&b.data, data, len(b.data))
}

func (b *FixedBytesValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	if len(b.data) == 0 {
		return fmt.Errorf("abi: invalid size, use NewFixedBytesType() to create a new FixedBytesValue")
	}
	var data []byte
	if err := m.MapRefl(src, reflect.ValueOf(&data)); err != nil {
		return err
	}
	return b.SetBytesPadRight(data)
}

func (b *FixedBytesValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	if len(b.data) == 0 {
		return fmt.Errorf("abi: invalid size, use NewFixedBytesType() to create a new FixedBytesValue")
	}
	return m.MapRefl(reflect.ValueOf(&b.data), dest)
}

type UintValue struct {
	val  *big.Int
	size int
}

func NewUintValue(size int) *UintValue {
	if size < 8 || size > 256 || size%8 != 0 {
		panic(fmt.Sprintf("abi: invalid size %d for uint", size))
	}
	return &UintValue{val: new(big.Int), size: size}
}

func (u *UintValue) Bytes() []byte {
	return u.val.Bytes()
}

func (u *UintValue) String() string {
	return u.val.String()
}

func (u *UintValue) Hex() string {
	return hexutil.BigIntToHex(u.val)
}

func (u *UintValue) BigInt() *big.Int {
	return u.val
}

func (u *UintValue) Uint64() (uint64, error) {
	if u.size > 64 {
		return 0, fmt.Errorf("abi: cannot convert uint%d to uint64", u.size)
	}
	return u.val.Uint64(), nil
}

func (u *UintValue) SetBytes(d []byte) error {
	if len(d)*8 > u.size {
		return fmt.Errorf("abi: cannot set %d bytes into uint%d", len(d), u.size)
	}
	u.val.SetBytes(d)
	return nil
}

func (u *UintValue) SetHex(s string) error {
	data, err := hexutil.HexToBytes(s)
	if err != nil {
		return err
	}
	return u.SetBytes(data)
}

func (u *UintValue) SetBigInt(i *big.Int) error {
	if i.BitLen() > u.size {
		return fmt.Errorf("abi: cannot set %d-bit integer into uint%d", i.BitLen(), u.size)
	}
	u.val.Set(i)
	return nil
}

func (u *UintValue) SetUint64(i uint64) error {
	return u.SetBigInt(new(big.Int).SetUint64(i))
}

func (u *UintValue) DynamicType() bool {
	return false
}

func (u *UintValue) EncodeABI() (Words, error) {
	if u.size == 0 {
		return nil, fmt.Errorf("abi: invalid size, use NewUintType() to create a new UintValue")
	}
	return encodeUint(u.val, u.size)
}

func (u *UintValue) DecodeABI(words Words) (int, error) {
	if u.size == 0 {
		return 0, fmt.Errorf("abi: invalid size, use NewUintType() to create a new UintValue")
	}
	return decodeUint(u.val, words)
}

func (u *UintValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	if u.size == 0 {
		return fmt.Errorf("abi: invalid size, use NewUintType() to create a new UintValue")
	}
	var val *big.Int
	if err := m.MapRefl(src, reflect.ValueOf(&val)); err != nil {
		return err
	}
	return u.SetBigInt(val)
}

func (u *UintValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	if u.size == 0 {
		return fmt.Errorf("abi: invalid size, use NewUintType() to create a new UintValue")
	}
	return m.MapRefl(reflect.ValueOf(&u.val), dest)
}

type IntValue struct {
	val  *big.Int
	size int
}

func NewIntValue(size int) *IntValue {
	if size < 8 || size > 256 || size%8 != 0 {
		panic(fmt.Sprintf("abi: invalid size %d for int", size))
	}
	return &IntValue{val: new(big.Int), size: size}
}

func (i *IntValue) Bytes() []byte {
	return i.val.Bytes()
}

func (i *IntValue) String() string {
	return i.val.String()
}

func (i *IntValue) Hex() string {
	return hexutil.BigIntToHex(i.val)
}

func (i *IntValue) BigInt() *big.Int {
	return i.val
}

func (i *IntValue) Int64() (int64, error) {
	if i.size > 64 {
		return 0, fmt.Errorf("abi: cannot convert int%d to int64", i.size)
	}
	return i.val.Int64(), nil
}

func (i *IntValue) SetBytes(d []byte) error {
	if len(d) > i.size {
		return fmt.Errorf("abi: cannot set %d bytes into int%d", len(d), i.size)
	}
	i.val.SetBytes(d)
	return nil
}

func (i *IntValue) SetHex(s string) error {
	data, err := hexutil.HexToBytes(s)
	if err != nil {
		return err
	}
	return i.SetBytes(data)
}

func (i *IntValue) SetBigInt(x *big.Int) error {
	if signedBitLen(x) > i.size {
		return fmt.Errorf("abi: cannot set %d-bit integer into int%d", x.BitLen(), i.size)
	}
	i.val.Set(x)
	return nil
}

func (i *IntValue) SetInt64(x int64) error {
	return i.SetBigInt(new(big.Int).SetInt64(x))
}

func (i *IntValue) DynamicType() bool {
	return false
}

func (i *IntValue) EncodeABI() (Words, error) {
	if i.size == 0 {
		return nil, fmt.Errorf("abi: invalid size, use NewIntType() to create a new IntValue")
	}
	return encodeInt(i.val, i.size)
}

func (i *IntValue) DecodeABI(words Words) (int, error) {
	if i.size == 0 {
		return 0, fmt.Errorf("abi: invalid size, use NewIntType() to create a new IntValue")
	}
	return decodeInt(i.val, words)
}

func (i *IntValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	if i.size == 0 {
		return fmt.Errorf("abi: invalid size, use NewIntType() to create a new IntValue")
	}
	var val *big.Int
	if err := m.MapRefl(src, reflect.ValueOf(&val)); err != nil {
		return err
	}
	return i.SetBigInt(val)
}

func (i *IntValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	if i.size == 0 {
		return fmt.Errorf("abi: invalid size, use NewIntType() to create a new IntValue")
	}
	return m.MapRefl(reflect.ValueOf(&i.val), dest)
}

type BoolValue bool

func NewBoolValue() *BoolValue {
	return new(BoolValue)
}

func (b *BoolValue) Bool() bool {
	return bool(*b)
}

func (b *BoolValue) SetBool(v bool) {
	*b = BoolValue(v)
}

func (b *BoolValue) DynamicType() bool {
	return false
}

func (b *BoolValue) EncodeABI() (Words, error) {
	return encodeBool(bool(*b)), nil
}

func (b *BoolValue) DecodeABI(words Words) (int, error) {
	return decodeBool((*bool)(b), words)
}

func (b *BoolValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	var val bool
	if err := m.MapRefl(src, reflect.ValueOf(&val)); err != nil {
		return err
	}
	*b = BoolValue(val)
	return nil
}

func (b *BoolValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	val := bool(*b)
	return m.MapRefl(reflect.ValueOf(val), dest)
}

type AddressValue types.Address

func NewAddressValue() *AddressValue {
	return new(AddressValue)
}

func (a *AddressValue) Address() types.Address {
	return types.Address(*a)
}

func (a *AddressValue) SetAddress(addr types.Address) {
	*a = AddressValue(addr)
}

func (a *AddressValue) DynamicType() bool {
	return false
}

func (a *AddressValue) EncodeABI() (Words, error) {
	var w Word
	copy(w[WordLength-types.AddressLength:], a[:])
	return Words{w}, nil
}

func (a *AddressValue) DecodeABI(words Words) (int, error) {
	if len(words) == 0 {
		return 0, fmt.Errorf("abi: cannot unmarshal address from empty value")
	}
	copy(a[:], words[0][WordLength-types.AddressLength:])
	return 1, nil
}

func (a *AddressValue) MapFrom(m *anymapper.Mapper, src reflect.Value) error {
	if !m.StrictTypes && src.Type().Kind() == reflect.String {
		addr, err := types.HexToAddress(src.String())
		if err != nil {
			return fmt.Errorf("abi: cannot convert string to address: %v", err)
		}
		*a = AddressValue(addr)
		return nil
	}
	var addr types.Address
	if err := m.MapRefl(src, reflect.ValueOf(&addr)); err != nil {
		return err
	}
	*a = AddressValue(addr)
	return nil
}

func (a *AddressValue) MapInto(m *anymapper.Mapper, dest reflect.Value) error {
	if !m.StrictTypes && dest.Type().Kind() == reflect.String {
		dest.SetString(a.Address().String())
		return nil
	}
	addr := types.Address(*a)
	return m.MapRefl(reflect.ValueOf(addr), dest)
}
