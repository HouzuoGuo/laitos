// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.6.1
// source: unlocksvc.proto

package unlocksvc

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// UnlockAttemptIdentification encapsulates identification properties of a program instance (the config/data files of which are currently password-locked)
// along with properties of its computer host.
type UnlockAttemptIdentification struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// HostName is the name of the computer host as seen by the kernel (kernel.hostname).
	HostName string `protobuf:"bytes,1,opt,name=HostName,proto3" json:"HostName,omitempty"`
	// PID is the program process ID.
	PID uint64 `protobuf:"varint,2,opt,name=PID,proto3" json:"PID,omitempty"`
	// RandomChallenge is a string of random characters generated when the program instance starts up. The string acts as a disposable secret to identify
	// this program instance.
	RandomChallenge string `protobuf:"bytes,3,opt,name=RandomChallenge,proto3" json:"RandomChallenge,omitempty"`
}

func (x *UnlockAttemptIdentification) Reset() {
	*x = UnlockAttemptIdentification{}
	if protoimpl.UnsafeEnabled {
		mi := &file_unlocksvc_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnlockAttemptIdentification) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnlockAttemptIdentification) ProtoMessage() {}

func (x *UnlockAttemptIdentification) ProtoReflect() protoreflect.Message {
	mi := &file_unlocksvc_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UnlockAttemptIdentification.ProtoReflect.Descriptor instead.
func (*UnlockAttemptIdentification) Descriptor() ([]byte, []int) {
	return file_unlocksvc_proto_rawDescGZIP(), []int{0}
}

func (x *UnlockAttemptIdentification) GetHostName() string {
	if x != nil {
		return x.HostName
	}
	return ""
}

func (x *UnlockAttemptIdentification) GetPID() uint64 {
	if x != nil {
		return x.PID
	}
	return 0
}

func (x *UnlockAttemptIdentification) GetRandomChallenge() string {
	if x != nil {
		return x.RandomChallenge
	}
	return ""
}

// PostUnlockIntentRequest provides input parameters for a program instance's intent of asking a user to provide the password for unlocking config/data files.
type PostUnlockIntentRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Identification *UnlockAttemptIdentification `protobuf:"bytes,1,opt,name=identification,proto3" json:"identification,omitempty"`
}

func (x *PostUnlockIntentRequest) Reset() {
	*x = PostUnlockIntentRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_unlocksvc_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PostUnlockIntentRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PostUnlockIntentRequest) ProtoMessage() {}

func (x *PostUnlockIntentRequest) ProtoReflect() protoreflect.Message {
	mi := &file_unlocksvc_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PostUnlockIntentRequest.ProtoReflect.Descriptor instead.
func (*PostUnlockIntentRequest) Descriptor() ([]byte, []int) {
	return file_unlocksvc_proto_rawDescGZIP(), []int{1}
}

func (x *PostUnlockIntentRequest) GetIdentification() *UnlockAttemptIdentification {
	if x != nil {
		return x.Identification
	}
	return nil
}

// PostUnlockIntentResponse represents a response from an intent of asking a user to provide the unlocking password.
type PostUnlockIntentResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PostUnlockIntentResponse) Reset() {
	*x = PostUnlockIntentResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_unlocksvc_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PostUnlockIntentResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PostUnlockIntentResponse) ProtoMessage() {}

func (x *PostUnlockIntentResponse) ProtoReflect() protoreflect.Message {
	mi := &file_unlocksvc_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PostUnlockIntentResponse.ProtoReflect.Descriptor instead.
func (*PostUnlockIntentResponse) Descriptor() ([]byte, []int) {
	return file_unlocksvc_proto_rawDescGZIP(), []int{2}
}

// GetUnlockPasswordRequest provides input parameters for a program instance to obtain unlocking password a user has already offered.
type GetUnlockPasswordRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Identification *UnlockAttemptIdentification `protobuf:"bytes,1,opt,name=identification,proto3" json:"identification,omitempty"`
}

func (x *GetUnlockPasswordRequest) Reset() {
	*x = GetUnlockPasswordRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_unlocksvc_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetUnlockPasswordRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetUnlockPasswordRequest) ProtoMessage() {}

func (x *GetUnlockPasswordRequest) ProtoReflect() protoreflect.Message {
	mi := &file_unlocksvc_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetUnlockPasswordRequest.ProtoReflect.Descriptor instead.
func (*GetUnlockPasswordRequest) Descriptor() ([]byte, []int) {
	return file_unlocksvc_proto_rawDescGZIP(), []int{3}
}

func (x *GetUnlockPasswordRequest) GetIdentification() *UnlockAttemptIdentification {
	if x != nil {
		return x.Identification
	}
	return nil
}

// GetUnlockPasswordRequest represents a response from obtaining the password offered by a user for unlocking config/data files.
type GetUnlockPasswordResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Exists is true only if a user has already offered a password for the unlocking of config/data files.
	Exists bool `protobuf:"varint,1,opt,name=Exists,proto3" json:"Exists,omitempty"`
	// Password is the password string used to unlock config/data files.
	Password string `protobuf:"bytes,2,opt,name=Password,proto3" json:"Password,omitempty"`
}

func (x *GetUnlockPasswordResponse) Reset() {
	*x = GetUnlockPasswordResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_unlocksvc_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetUnlockPasswordResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetUnlockPasswordResponse) ProtoMessage() {}

func (x *GetUnlockPasswordResponse) ProtoReflect() protoreflect.Message {
	mi := &file_unlocksvc_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetUnlockPasswordResponse.ProtoReflect.Descriptor instead.
func (*GetUnlockPasswordResponse) Descriptor() ([]byte, []int) {
	return file_unlocksvc_proto_rawDescGZIP(), []int{4}
}

func (x *GetUnlockPasswordResponse) GetExists() bool {
	if x != nil {
		return x.Exists
	}
	return false
}

func (x *GetUnlockPasswordResponse) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

var File_unlocksvc_proto protoreflect.FileDescriptor

var file_unlocksvc_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x75, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x13, 0x68, 0x7a, 0x67, 0x6c, 0x6c, 0x61, 0x69, 0x74, 0x6f, 0x73, 0x75, 0x6e, 0x6c,
	0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x22, 0x75, 0x0a, 0x1b, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b,
	0x41, 0x74, 0x74, 0x65, 0x6d, 0x70, 0x74, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x63,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x48, 0x6f, 0x73, 0x74, 0x4e, 0x61, 0x6d,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x48, 0x6f, 0x73, 0x74, 0x4e, 0x61, 0x6d,
	0x65, 0x12, 0x10, 0x0a, 0x03, 0x50, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x03,
	0x50, 0x49, 0x44, 0x12, 0x28, 0x0a, 0x0f, 0x52, 0x61, 0x6e, 0x64, 0x6f, 0x6d, 0x43, 0x68, 0x61,
	0x6c, 0x6c, 0x65, 0x6e, 0x67, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x52, 0x61,
	0x6e, 0x64, 0x6f, 0x6d, 0x43, 0x68, 0x61, 0x6c, 0x6c, 0x65, 0x6e, 0x67, 0x65, 0x22, 0x73, 0x0a,
	0x17, 0x50, 0x6f, 0x73, 0x74, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x49, 0x6e, 0x74, 0x65, 0x6e,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x58, 0x0a, 0x0e, 0x69, 0x64, 0x65, 0x6e,
	0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x30, 0x2e, 0x68, 0x7a, 0x67, 0x6c, 0x6c, 0x61, 0x69, 0x74, 0x6f, 0x73, 0x75, 0x6e, 0x6c,
	0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x2e, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x41, 0x74, 0x74,
	0x65, 0x6d, 0x70, 0x74, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x0e, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x22, 0x1a, 0x0a, 0x18, 0x50, 0x6f, 0x73, 0x74, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b,
	0x49, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x74,
	0x0a, 0x18, 0x47, 0x65, 0x74, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x61, 0x73, 0x73, 0x77,
	0x6f, 0x72, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x58, 0x0a, 0x0e, 0x69, 0x64,
	0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x30, 0x2e, 0x68, 0x7a, 0x67, 0x6c, 0x6c, 0x61, 0x69, 0x74, 0x6f, 0x73, 0x75,
	0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x2e, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x41,
	0x74, 0x74, 0x65, 0x6d, 0x70, 0x74, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0e, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x22, 0x4f, 0x0a, 0x19, 0x47, 0x65, 0x74, 0x55, 0x6e, 0x6c, 0x6f, 0x63,
	0x6b, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x16, 0x0a, 0x06, 0x45, 0x78, 0x69, 0x73, 0x74, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x06, 0x45, 0x78, 0x69, 0x73, 0x74, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x50, 0x61, 0x73,
	0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x50, 0x61, 0x73,
	0x73, 0x77, 0x6f, 0x72, 0x64, 0x32, 0xfc, 0x01, 0x0a, 0x15, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f,
	0x72, 0x64, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12,
	0x6f, 0x0a, 0x10, 0x50, 0x6f, 0x73, 0x74, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x49, 0x6e, 0x74,
	0x65, 0x6e, 0x74, 0x12, 0x2c, 0x2e, 0x68, 0x7a, 0x67, 0x6c, 0x6c, 0x61, 0x69, 0x74, 0x6f, 0x73,
	0x75, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x2e, 0x50, 0x6f, 0x73, 0x74, 0x55, 0x6e,
	0x6c, 0x6f, 0x63, 0x6b, 0x49, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x2d, 0x2e, 0x68, 0x7a, 0x67, 0x6c, 0x6c, 0x61, 0x69, 0x74, 0x6f, 0x73, 0x75, 0x6e,
	0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x2e, 0x50, 0x6f, 0x73, 0x74, 0x55, 0x6e, 0x6c, 0x6f,
	0x63, 0x6b, 0x49, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x72, 0x0a, 0x11, 0x47, 0x65, 0x74, 0x55, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x61, 0x73,
	0x73, 0x77, 0x6f, 0x72, 0x64, 0x12, 0x2d, 0x2e, 0x68, 0x7a, 0x67, 0x6c, 0x6c, 0x61, 0x69, 0x74,
	0x6f, 0x73, 0x75, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x2e, 0x47, 0x65, 0x74, 0x55,
	0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x2e, 0x2e, 0x68, 0x7a, 0x67, 0x6c, 0x6c, 0x61, 0x69, 0x74, 0x6f,
	0x73, 0x75, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x2e, 0x47, 0x65, 0x74, 0x55, 0x6e,
	0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x42, 0x27, 0x5a, 0x25, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x48, 0x6f, 0x75, 0x7a, 0x75, 0x6f, 0x47, 0x75, 0x6f, 0x2f, 0x6c, 0x61, 0x69,
	0x74, 0x6f, 0x73, 0x3b, 0x75, 0x6e, 0x6c, 0x6f, 0x63, 0x6b, 0x73, 0x76, 0x63, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_unlocksvc_proto_rawDescOnce sync.Once
	file_unlocksvc_proto_rawDescData = file_unlocksvc_proto_rawDesc
)

func file_unlocksvc_proto_rawDescGZIP() []byte {
	file_unlocksvc_proto_rawDescOnce.Do(func() {
		file_unlocksvc_proto_rawDescData = protoimpl.X.CompressGZIP(file_unlocksvc_proto_rawDescData)
	})
	return file_unlocksvc_proto_rawDescData
}

var file_unlocksvc_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_unlocksvc_proto_goTypes = []interface{}{
	(*UnlockAttemptIdentification)(nil), // 0: hzgllaitosunlocksvc.UnlockAttemptIdentification
	(*PostUnlockIntentRequest)(nil),     // 1: hzgllaitosunlocksvc.PostUnlockIntentRequest
	(*PostUnlockIntentResponse)(nil),    // 2: hzgllaitosunlocksvc.PostUnlockIntentResponse
	(*GetUnlockPasswordRequest)(nil),    // 3: hzgllaitosunlocksvc.GetUnlockPasswordRequest
	(*GetUnlockPasswordResponse)(nil),   // 4: hzgllaitosunlocksvc.GetUnlockPasswordResponse
}
var file_unlocksvc_proto_depIdxs = []int32{
	0, // 0: hzgllaitosunlocksvc.PostUnlockIntentRequest.identification:type_name -> hzgllaitosunlocksvc.UnlockAttemptIdentification
	0, // 1: hzgllaitosunlocksvc.GetUnlockPasswordRequest.identification:type_name -> hzgllaitosunlocksvc.UnlockAttemptIdentification
	1, // 2: hzgllaitosunlocksvc.PasswordUnlockService.PostUnlockIntent:input_type -> hzgllaitosunlocksvc.PostUnlockIntentRequest
	3, // 3: hzgllaitosunlocksvc.PasswordUnlockService.GetUnlockPassword:input_type -> hzgllaitosunlocksvc.GetUnlockPasswordRequest
	2, // 4: hzgllaitosunlocksvc.PasswordUnlockService.PostUnlockIntent:output_type -> hzgllaitosunlocksvc.PostUnlockIntentResponse
	4, // 5: hzgllaitosunlocksvc.PasswordUnlockService.GetUnlockPassword:output_type -> hzgllaitosunlocksvc.GetUnlockPasswordResponse
	4, // [4:6] is the sub-list for method output_type
	2, // [2:4] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_unlocksvc_proto_init() }
func file_unlocksvc_proto_init() {
	if File_unlocksvc_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_unlocksvc_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UnlockAttemptIdentification); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_unlocksvc_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PostUnlockIntentRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_unlocksvc_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PostUnlockIntentResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_unlocksvc_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetUnlockPasswordRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_unlocksvc_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetUnlockPasswordResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_unlocksvc_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_unlocksvc_proto_goTypes,
		DependencyIndexes: file_unlocksvc_proto_depIdxs,
		MessageInfos:      file_unlocksvc_proto_msgTypes,
	}.Build()
	File_unlocksvc_proto = out.File
	file_unlocksvc_proto_rawDesc = nil
	file_unlocksvc_proto_goTypes = nil
	file_unlocksvc_proto_depIdxs = nil
}
