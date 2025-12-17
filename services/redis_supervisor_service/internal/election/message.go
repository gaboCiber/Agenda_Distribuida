package election

import (
    "context"
    "fmt"

    "google.golang.org/grpc"
)

// MessageType represents the type of election message.
type MessageType int32

const (
    MessageType_UNKNOWN     MessageType = 0
    MessageType_ELECTION    MessageType = 1
    MessageType_OK          MessageType = 2
    MessageType_COORDINATOR MessageType = 3
    MessageType_HEARTBEAT   MessageType = 4
)

// ElectionMessage is exchanged between supervisor peers.
type ElectionMessage struct {
    Type        MessageType `protobuf:"varint,1,opt,name=type,proto3" json:"type,omitempty"`
    SenderId    string      `protobuf:"bytes,2,opt,name=sender_id,json=senderId,proto3" json:"sender_id,omitempty"`
    Epoch       uint64      `protobuf:"varint,3,opt,name=epoch,proto3" json:"epoch,omitempty"`
    LeaderId    string      `protobuf:"bytes,4,opt,name=leader_id,json=leaderId,proto3" json:"leader_id,omitempty"`
    PrimaryAddr string      `protobuf:"bytes,5,opt,name=primary_addr,json=primaryAddr,proto3" json:"primary_addr,omitempty"`
}

// Reset implements proto.Message.
func (m *ElectionMessage) Reset() { *m = ElectionMessage{} }

// String implements proto.Message.
func (m *ElectionMessage) String() string {
    return fmt.Sprintf("ElectionMessage{Type:%d, Sender:%s, Epoch:%d, Leader:%s, Primary:%s}", m.Type, m.SenderId, m.Epoch, m.LeaderId, m.PrimaryAddr)
}

// ProtoMessage marks the struct as compatible with protobuf encoding.
func (*ElectionMessage) ProtoMessage() {}

// ElectionServiceClient is the client API for the election gRPC service.
type ElectionServiceClient interface {
    SendMessage(ctx context.Context, in *ElectionMessage, opts ...grpc.CallOption) (*ElectionMessage, error)
}

type electionServiceClient struct {
    cc grpc.ClientConnInterface
}

// NewElectionServiceClient creates a new gRPC client for the election service.
func NewElectionServiceClient(cc grpc.ClientConnInterface) ElectionServiceClient {
    return &electionServiceClient{cc: cc}
}

func (c *electionServiceClient) SendMessage(ctx context.Context, in *ElectionMessage, opts ...grpc.CallOption) (*ElectionMessage, error) {
    out := new(ElectionMessage)
    err := c.cc.Invoke(ctx, "/election.ElectionService/SendMessage", in, out, opts...)
    if err != nil {
        return nil, err
    }
    return out, nil
}

// ElectionServiceServer defines the server API for the election service.
type ElectionServiceServer interface {
    SendMessage(context.Context, *ElectionMessage) (*ElectionMessage, error)
}

// UnimplementedElectionServiceServer can be embedded to have forward compatible implementations.
type UnimplementedElectionServiceServer struct{}

// SendMessage is a stub implementation.
func (UnimplementedElectionServiceServer) SendMessage(context.Context, *ElectionMessage) (*ElectionMessage, error) {
    return nil, fmt.Errorf("method SendMessage not implemented")
}

// RegisterElectionServiceServer registers the service implementation with a gRPC server.
func RegisterElectionServiceServer(s *grpc.Server, srv ElectionServiceServer) {
    s.RegisterService(&_ElectionService_serviceDesc, srv)
}

func _ElectionService_SendMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := new(ElectionMessage)
    if err := dec(in); err != nil {
        return nil, err
    }
    if interceptor == nil {
        return srv.(ElectionServiceServer).SendMessage(ctx, in)
    }
    info := &grpc.UnaryServerInfo{
        Server:     srv,
        FullMethod: "/election.ElectionService/SendMessage",
    }
    handler := func(ctx context.Context, req interface{}) (interface{}, error) {
        return srv.(ElectionServiceServer).SendMessage(ctx, req.(*ElectionMessage))
    }
    return interceptor(ctx, in, info, handler)
}

var _ElectionService_serviceDesc = grpc.ServiceDesc{
    ServiceName: "election.ElectionService",
    HandlerType: (*ElectionServiceServer)(nil),
    Methods: []grpc.MethodDesc{
        {
            MethodName: "SendMessage",
            Handler:    _ElectionService_SendMessage_Handler,
        },
    },
    Streams:  []grpc.StreamDesc{},
    Metadata: "internal/election/message.go",
}
