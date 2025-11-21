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

	"google.golang.org/grpc"
)

// NewPluginStreamClientWithAdapter creates a PluginStreamClient with a BidiStreamAdapter in one call.
// This is a convenience function that combines adapter creation and client initialization.
//
// Parameters:
//   - ctx: context for initialization
//   - stream: the underlying gRPC bidirectional stream (domain-specific message type)
//   - pluginName: name of the plugin
//   - pluginVersion: version of the plugin
//   - service: gRPC service descriptor for routing RPC calls
//   - impl: implementation of the gRPC service
//   - wrapMessage: function to wrap framework message bytes into domain message
//   - unwrapMessage: function to extract bytes from domain message
//
// Example usage:
//
//	client, err := stream.NewPluginStreamClientWithAdapter(
//	    ctx,
//	    grpcStream,
//	    "my-plugin",
//	    "v1.0.0",
//	    &pb.MyService_ServiceDesc,
//	    &myServiceImpl{},
//	    func(data []byte) *pb.MyMessage { return &pb.MyMessage{Data: data} },
//	    func(msg *pb.MyMessage) []byte { return msg.GetData() },
//	)
func NewPluginStreamClientWithAdapter[T MessageWrapper](
	ctx context.Context,
	stream interface {
		Send(T) error
		Recv() (T, error)
		Context() context.Context
	},
	pluginName string,
	pluginVersion string,
	service grpc.ServiceDesc,
	impl interface{},
	wrapMessage func([]byte) T,
	unwrapMessage func(T) []byte,
) (*PluginStreamClient, error) {
	// Create adapter
	adaptedStream := NewBidiStreamAdapter(stream, wrapMessage, unwrapMessage)

	// Create and return client
	return NewPluginStreamClient(ctx, adaptedStream, pluginName, pluginVersion, service, impl)
}
