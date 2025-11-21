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
	"path"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	pluginframeworkv1 "github.com/guilhem/operator-plugin-framework/pluginframework/v1"
)

// PluginStreamClient manages the plugin side of the bidirectional stream.
// It handles registration, receives RPC calls from the operator, and sends responses back.
type PluginStreamClient struct {
	stream     StreamInterface
	pluginName string
	pluginVer  string
	service    grpc.ServiceDesc
	impl       any
}

// NewPluginStreamClient creates a new PluginStreamClient and sends the registration message.
func NewPluginStreamClient(
	ctx context.Context,
	stream StreamInterface,
	pluginName string,
	pluginVersion string,
	service grpc.ServiceDesc,
	impl any,
) (*PluginStreamClient, error) {
	psc := &PluginStreamClient{
		stream:     stream,
		pluginName: pluginName,
		pluginVer:  pluginVersion,
		service:    service,
		impl:       impl,
	}

	// Send registration message
	registerMsg := &pluginframeworkv1.PluginStreamMessage{
		Payload: &pluginframeworkv1.PluginStreamMessage_Register{
			Register: &pluginframeworkv1.PluginRegister{
				Name:    pluginName,
				Version: pluginVersion,
			},
		},
	}

	if err := stream.Send(registerMsg); err != nil {
		return nil, fmt.Errorf("failed to send registration: %w", err)
	}

	return psc, nil
}

// HandleRPCCalls continuously listens for RPC calls from the operator and processes them using the handler.
// This should be run in the main goroutine or as the primary loop of the plugin.
func (psc *PluginStreamClient) HandleRPCCalls(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			// Try to close the stream if it has a CloseSend method
			if closer, ok := psc.stream.(interface{ CloseSend() error }); ok {
				return closer.CloseSend()
			}
			return nil
		default:
		}

		msg, err := psc.stream.Recv()
		if err != nil {
			return fmt.Errorf("failed to receive message: %w", err)
		}

		// Handle RPC call
		rpcCall := msg.GetRpcCall()
		if rpcCall != nil {
			go func() {
				if err := psc.handleRPCCall(ctx, rpcCall); err != nil {
					// Log the error since it's in a goroutine
					// Note: In a real implementation, you might want to use a logger
					fmt.Printf("Error handling RPC call: %v\n", err)
				}
			}()
		}
	}
}

// handleRPCCall processes a single RPC call from the operator.
func (psc *PluginStreamClient) handleRPCCall(ctx context.Context, rpcCall *pluginframeworkv1.PluginRPCCall) error {
	requestID := rpcCall.GetRequestId()
	fullMethod := rpcCall.GetMethod()
	method := path.Base(fullMethod)

	for _, m := range psc.service.Methods {
		if m.MethodName == method {
			dec := func(v interface{}) error {
				return proto.Unmarshal(rpcCall.GetPayload(), v.(proto.Message))
			}
			out, err := m.Handler(psc.impl, ctx, dec, nil)
			if err != nil {
				msg := &pluginframeworkv1.PluginStreamMessage{
					Payload: &pluginframeworkv1.PluginStreamMessage_Error{
						Error: &pluginframeworkv1.PluginError{
							Code:    "RPC_ERROR",
							Message: err.Error(),
						},
					},
				}
				return psc.stream.Send(msg)
			}
			respBytes, err := proto.Marshal(out.(proto.Message))
			if err != nil {
				msg := &pluginframeworkv1.PluginStreamMessage{
					Payload: &pluginframeworkv1.PluginStreamMessage_Error{
						Error: &pluginframeworkv1.PluginError{
							Code:    "MARSHAL_ERROR",
							Message: fmt.Sprintf("failed to marshal response: %v", err),
						},
					},
				}
				return psc.stream.Send(msg)
			}
			msg := &pluginframeworkv1.PluginStreamMessage{
				Payload: &pluginframeworkv1.PluginStreamMessage_RpcResponse{
					RpcResponse: &pluginframeworkv1.PluginRPCResponse{
						RequestId: requestID,
						Payload:   respBytes,
					},
				},
			}
			return psc.stream.Send(msg)
		}
	}

	msg := &pluginframeworkv1.PluginStreamMessage{
		Payload: &pluginframeworkv1.PluginStreamMessage_Error{
			Error: &pluginframeworkv1.PluginError{
				Code:    codes.Unimplemented.String(),
				Message: fmt.Sprintf("unknown method %s", fullMethod),
			},
		},
	}
	return psc.stream.Send(msg)
}
