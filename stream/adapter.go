/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package stream

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	pluginframeworkv1 "github.com/guilhem/operator-plugin-framework/pluginframework/v1"
)

// MessageWrapper is an interface for any protobuf message type that can carry
// serialized PluginStreamMessage data (e.g., as a bytes field).
type MessageWrapper interface {
	proto.Message
	GetData() []byte
}

// BidiStreamAdapter adapts a gRPC bidirectional stream using a wrapper message type
// to the framework's PluginStreamMessage protocol.
//
// This is useful when you need to embed PluginStreamMessage within another protobuf message
// (e.g., because your gRPC service definition uses a domain-specific message type).
type BidiStreamAdapter[T MessageWrapper] struct {
	stream interface {
		Send(T) error
		Recv() (T, error)
		Context() context.Context
	}
	wrapMessage   func([]byte) T
	unwrapMessage func(T) []byte
}

// NewBidiStreamAdapter creates a new adapter for a bidirectional stream.
// The wrapMessage function should create a wrapper message with the given data bytes.
// The unwrapMessage function should extract the data bytes from a wrapper message.
//
// Example usage:
//
//	adapter := stream.NewBidiStreamAdapter(
//	    grpcStream,
//	    func(data []byte) *pb.MyWrapperMessage { return &pb.MyWrapperMessage{Data: data} },
//	    func(msg *pb.MyWrapperMessage) []byte { return msg.GetData() },
//	)
func NewBidiStreamAdapter[T MessageWrapper](
	stream interface {
		Send(T) error
		Recv() (T, error)
		Context() context.Context
	},
	wrapMessage func([]byte) T,
	unwrapMessage func(T) []byte,
) *BidiStreamAdapter[T] {
	return &BidiStreamAdapter[T]{
		stream:        stream,
		wrapMessage:   wrapMessage,
		unwrapMessage: unwrapMessage,
	}
}

func (bsa *BidiStreamAdapter[T]) Send(msg *pluginframeworkv1.PluginStreamMessage) error {
	// Marshal framework message to bytes
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal framework message: %w", err)
	}

	// Wrap in domain-specific message
	wrapper := bsa.wrapMessage(msgBytes)

	return bsa.stream.Send(wrapper)
}

func (bsa *BidiStreamAdapter[T]) Recv() (*pluginframeworkv1.PluginStreamMessage, error) {
	// Receive wrapper message
	wrapper, err := bsa.stream.Recv()
	if err != nil {
		return nil, err
	}

	// Extract data bytes
	msgBytes := bsa.unwrapMessage(wrapper)

	// Unmarshal to framework message
	frameworkMsg := &pluginframeworkv1.PluginStreamMessage{}
	if err := proto.Unmarshal(msgBytes, frameworkMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to framework message: %w", err)
	}

	return frameworkMsg, nil
}

func (bsa *BidiStreamAdapter[T]) Context() context.Context {
	return bsa.stream.Context()
}
