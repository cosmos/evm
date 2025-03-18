// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: cosmos/evm/erc20/v1/genesis.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/cosmos-sdk/types/tx/amino"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// GenesisState defines the module's genesis state.
type GenesisState struct {
	// params are the erc20 module parameters at genesis
	Params Params `protobuf:"bytes,1,opt,name=params,proto3" json:"params"`
	// token_pairs is a slice of the registered token pairs at genesis
	TokenPairs []TokenPair `protobuf:"bytes,2,rep,name=token_pairs,json=tokenPairs,proto3" json:"token_pairs"`
}

func (m *GenesisState) Reset()         { *m = GenesisState{} }
func (m *GenesisState) String() string { return proto.CompactTextString(m) }
func (*GenesisState) ProtoMessage()    {}
func (*GenesisState) Descriptor() ([]byte, []int) {
	return fileDescriptor_e964b7a0cc2cbbd5, []int{0}
}
func (m *GenesisState) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GenesisState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GenesisState.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GenesisState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GenesisState.Merge(m, src)
}
func (m *GenesisState) XXX_Size() int {
	return m.Size()
}
func (m *GenesisState) XXX_DiscardUnknown() {
	xxx_messageInfo_GenesisState.DiscardUnknown(m)
}

var xxx_messageInfo_GenesisState proto.InternalMessageInfo

func (m *GenesisState) GetParams() Params {
	if m != nil {
		return m.Params
	}
	return Params{}
}

func (m *GenesisState) GetTokenPairs() []TokenPair {
	if m != nil {
		return m.TokenPairs
	}
	return nil
}

// Params defines the erc20 module params
type Params struct {
	// enable_erc20 is the parameter to enable the conversion of Cosmos coins <-->
	// ERC20 tokens.
	EnableErc20 bool `protobuf:"varint,1,opt,name=enable_erc20,json=enableErc20,proto3" json:"enable_erc20,omitempty"`
	// native_precompiles defines the slice of hex addresses of the
	// active precompiles that are used to interact with native staking coins as
	// ERC20s
	NativePrecompiles []string `protobuf:"bytes,3,rep,name=native_precompiles,json=nativePrecompiles,proto3" json:"native_precompiles,omitempty"`
	// dynamic_precompiles defines the slice of hex addresses of the
	// active precompiles that are used to interact with Bank coins as ERC20s
	DynamicPrecompiles []string `protobuf:"bytes,4,rep,name=dynamic_precompiles,json=dynamicPrecompiles,proto3" json:"dynamic_precompiles,omitempty"`
}

func (m *Params) Reset()         { *m = Params{} }
func (m *Params) String() string { return proto.CompactTextString(m) }
func (*Params) ProtoMessage()    {}
func (*Params) Descriptor() ([]byte, []int) {
	return fileDescriptor_e964b7a0cc2cbbd5, []int{1}
}
func (m *Params) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Params) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Params.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Params) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Params.Merge(m, src)
}
func (m *Params) XXX_Size() int {
	return m.Size()
}
func (m *Params) XXX_DiscardUnknown() {
	xxx_messageInfo_Params.DiscardUnknown(m)
}

var xxx_messageInfo_Params proto.InternalMessageInfo

func (m *Params) GetEnableErc20() bool {
	if m != nil {
		return m.EnableErc20
	}
	return false
}

func (m *Params) GetNativePrecompiles() []string {
	if m != nil {
		return m.NativePrecompiles
	}
	return nil
}

func (m *Params) GetDynamicPrecompiles() []string {
	if m != nil {
		return m.DynamicPrecompiles
	}
	return nil
}

func init() {
	proto.RegisterType((*GenesisState)(nil), "cosmos.evm.erc20.v1.GenesisState")
	proto.RegisterType((*Params)(nil), "cosmos.evm.erc20.v1.Params")
}

func init() { proto.RegisterFile("cosmos/evm/erc20/v1/genesis.proto", fileDescriptor_e964b7a0cc2cbbd5) }

var fileDescriptor_e964b7a0cc2cbbd5 = []byte{
	// 343 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x90, 0xcf, 0x4e, 0x32, 0x31,
	0x14, 0xc5, 0xa7, 0x0c, 0x21, 0xd0, 0x61, 0xf1, 0x51, 0xbe, 0x05, 0xc1, 0xa4, 0xfc, 0x71, 0x43,
	0x4c, 0x6c, 0x05, 0xd7, 0xba, 0x20, 0x31, 0x26, 0xac, 0x08, 0xba, 0x72, 0x43, 0xca, 0xd8, 0x8c,
	0x8d, 0x74, 0x3a, 0x99, 0xd6, 0x89, 0xbc, 0x85, 0x6e, 0x7d, 0x02, 0x97, 0x3e, 0x06, 0x4b, 0x96,
	0xae, 0x8c, 0x81, 0x85, 0xaf, 0x61, 0x68, 0xc7, 0x88, 0x09, 0x9b, 0xa6, 0x39, 0xf7, 0x77, 0xcf,
	0xb9, 0x39, 0xb0, 0x13, 0x2a, 0x2d, 0x95, 0xa6, 0x3c, 0x93, 0x94, 0xa7, 0xe1, 0xe0, 0x84, 0x66,
	0x7d, 0x1a, 0xf1, 0x98, 0x6b, 0xa1, 0x49, 0x92, 0x2a, 0xa3, 0x50, 0xdd, 0x21, 0x84, 0x67, 0x92,
	0x58, 0x84, 0x64, 0xfd, 0x66, 0x8d, 0x49, 0x11, 0x2b, 0x6a, 0x5f, 0xc7, 0x35, 0xff, 0x47, 0x2a,
	0x52, 0xf6, 0x4b, 0xb7, 0xbf, 0x5c, 0x6d, 0xed, 0x0b, 0x70, 0x36, 0x16, 0xe8, 0xbe, 0x00, 0x58,
	0xbd, 0x74, 0x81, 0x57, 0x86, 0x19, 0x8e, 0xce, 0x61, 0x29, 0x61, 0x29, 0x93, 0xba, 0x01, 0xda,
	0xa0, 0x17, 0x0c, 0x0e, 0xc8, 0x9e, 0x03, 0xc8, 0xd8, 0x22, 0xc3, 0xca, 0xf2, 0xa3, 0xe5, 0xbd,
	0x7e, 0xbd, 0x1d, 0x81, 0x49, 0xbe, 0x85, 0x46, 0x30, 0x30, 0xea, 0x9e, 0xc7, 0xd3, 0x84, 0x89,
	0x54, 0x37, 0x0a, 0x6d, 0xbf, 0x17, 0x0c, 0xf0, 0x5e, 0x93, 0xeb, 0x2d, 0x37, 0x66, 0x22, 0xdd,
	0xf5, 0x81, 0xe6, 0x47, 0xd5, 0xdd, 0x67, 0x00, 0x4b, 0x2e, 0x09, 0x75, 0x60, 0x95, 0xc7, 0x6c,
	0x36, 0xe7, 0x53, 0xbb, 0x6e, 0x8f, 0x2b, 0x4f, 0x02, 0xa7, 0x5d, 0x6c, 0x25, 0x74, 0x0c, 0x51,
	0xcc, 0x8c, 0xc8, 0xf8, 0x34, 0x49, 0x79, 0xa8, 0x64, 0x22, 0xe6, 0x5c, 0x37, 0xfc, 0xb6, 0xdf,
	0xab, 0x4c, 0x6a, 0x6e, 0x32, 0xfe, 0x1d, 0x20, 0x0a, 0xeb, 0xb7, 0x8b, 0x98, 0x49, 0x11, 0xfe,
	0xe1, 0x8b, 0x96, 0x47, 0xf9, 0x68, 0x67, 0x61, 0x54, 0x2c, 0x17, 0xfe, 0xf9, 0xc3, 0xb3, 0xe5,
	0x1a, 0x83, 0xd5, 0x1a, 0x83, 0xcf, 0x35, 0x06, 0x4f, 0x1b, 0xec, 0xad, 0x36, 0xd8, 0x7b, 0xdf,
	0x60, 0xef, 0xe6, 0x30, 0x12, 0xe6, 0xee, 0x61, 0x46, 0x42, 0x25, 0xe9, 0x4e, 0xed, 0x8f, 0x79,
	0xf1, 0x66, 0x91, 0x70, 0x3d, 0x2b, 0xd9, 0xda, 0x4f, 0xbf, 0x03, 0x00, 0x00, 0xff, 0xff, 0x59,
	0x13, 0x2d, 0x6f, 0xfa, 0x01, 0x00, 0x00,
}

func (m *GenesisState) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GenesisState) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GenesisState) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.TokenPairs) > 0 {
		for iNdEx := len(m.TokenPairs) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.TokenPairs[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintGenesis(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	{
		size, err := m.Params.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintGenesis(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func (m *Params) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Params) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Params) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.DynamicPrecompiles) > 0 {
		for iNdEx := len(m.DynamicPrecompiles) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.DynamicPrecompiles[iNdEx])
			copy(dAtA[i:], m.DynamicPrecompiles[iNdEx])
			i = encodeVarintGenesis(dAtA, i, uint64(len(m.DynamicPrecompiles[iNdEx])))
			i--
			dAtA[i] = 0x22
		}
	}
	if len(m.NativePrecompiles) > 0 {
		for iNdEx := len(m.NativePrecompiles) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.NativePrecompiles[iNdEx])
			copy(dAtA[i:], m.NativePrecompiles[iNdEx])
			i = encodeVarintGenesis(dAtA, i, uint64(len(m.NativePrecompiles[iNdEx])))
			i--
			dAtA[i] = 0x1a
		}
	}
	if m.EnableErc20 {
		i--
		if m.EnableErc20 {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintGenesis(dAtA []byte, offset int, v uint64) int {
	offset -= sovGenesis(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *GenesisState) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.Params.Size()
	n += 1 + l + sovGenesis(uint64(l))
	if len(m.TokenPairs) > 0 {
		for _, e := range m.TokenPairs {
			l = e.Size()
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	return n
}

func (m *Params) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.EnableErc20 {
		n += 2
	}
	if len(m.NativePrecompiles) > 0 {
		for _, s := range m.NativePrecompiles {
			l = len(s)
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	if len(m.DynamicPrecompiles) > 0 {
		for _, s := range m.DynamicPrecompiles {
			l = len(s)
			n += 1 + l + sovGenesis(uint64(l))
		}
	}
	return n
}

func sovGenesis(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozGenesis(x uint64) (n int) {
	return sovGenesis(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *GenesisState) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenesis
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: GenesisState: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GenesisState: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Params", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Params.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TokenPairs", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TokenPairs = append(m.TokenPairs, TokenPair{})
			if err := m.TokenPairs[len(m.TokenPairs)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenesis(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenesis
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Params) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowGenesis
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Params: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Params: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EnableErc20", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.EnableErc20 = bool(v != 0)
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field NativePrecompiles", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.NativePrecompiles = append(m.NativePrecompiles, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DynamicPrecompiles", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthGenesis
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthGenesis
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DynamicPrecompiles = append(m.DynamicPrecompiles, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipGenesis(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthGenesis
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipGenesis(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowGenesis
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowGenesis
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthGenesis
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupGenesis
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthGenesis
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthGenesis        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowGenesis          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupGenesis = fmt.Errorf("proto: unexpected end of group")
)
