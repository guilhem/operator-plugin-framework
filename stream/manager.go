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
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	pluginframeworkv1 "github.com/guilhem/operator-plugin-framework/pluginframework/v1"
)

// StreamManager manages bidirectional gRPC streams with plugins.
// It handles plugin registration, RPC call forwarding, and response correlation.
type StreamManager struct {
	stream     StreamInterface
	pluginName string
	pluginVer  string

	// Map to track pending RPC calls by request ID
	requestsMu   sync.RWMutex
	pendingCalls map[string]chan interface{}
}

// StreamInterface defines the minimal interface required for bidirectional streaming.
type StreamInterface interface {
	Send(*pluginframeworkv1.PluginStreamMessage) error
	Recv() (*pluginframeworkv1.PluginStreamMessage, error)
	Context() context.Context
}

// NewStreamManager creates a new StreamManager from a bidirectional stream.
// It expects the first message to be a PluginRegister message.
func NewStreamManager(
	stream StreamInterface,
) (*StreamManager, error) {
	// Wait for the first message (plugin registration)
	msg, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive plugin register: %w", err)
	}

	register := msg.GetRegister()
	if register == nil {
		return nil, fmt.Errorf("first message must be PluginRegister")
	}

	sm := &StreamManager{
		stream:       stream,
		pluginName:   register.Name,
		pluginVer:    register.Version,
		pendingCalls: make(map[string]chan interface{}),
	}

	return sm, nil
}

// GetPluginName returns the name of the registered plugin.
func (sm *StreamManager) GetPluginName() string {
	return sm.pluginName
}

// GetPluginVersion returns the version of the registered plugin.
func (sm *StreamManager) GetPluginVersion() string {
	return sm.pluginVer
}

// CallRPC sends an RPC call to the plugin and waits for the response.
// The method name and payload are protocol-specific (e.g., "RenewToken" with RenewTokenRequest).
// The response is returned as raw bytes that must be unmarshaled by the caller.
func (sm *StreamManager) CallRPC(ctx context.Context, method string, reqPayload proto.Message) ([]byte, error) {
	// Marshal request
	reqBytes, err := proto.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send RPC call
	requestID := generateRequestID()
	rpcCall := &pluginframeworkv1.PluginRPCCall{
		RequestId: requestID,
		Method:    method,
		Payload:   reqBytes,
	}

	msg := &pluginframeworkv1.PluginStreamMessage{
		Payload: &pluginframeworkv1.PluginStreamMessage_RpcCall{
			RpcCall: rpcCall,
		},
	}

	if err := sm.stream.Send(msg); err != nil {
		return nil, fmt.Errorf("failed to send RPC call: %w", err)
	}

	// Wait for response
	resp, err := sm.waitForResponse(ctx, requestID)
	if err != nil {
		return nil, err
	}

	respBytes, ok := resp.([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}

	return respBytes, nil
}

// ListenForMessages listens for incoming messages from the plugin (responses and errors).
// This should be run in a goroutine to continuously process plugin messages.
// It returns when the stream is closed or an error occurs.
func (sm *StreamManager) ListenForMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := sm.stream.Recv()
		if err != nil {
			return fmt.Errorf("failed to receive message from plugin: %w", err)
		}

		// Handle response message
		resp := msg.GetRpcResponse()
		if resp != nil {
			if err := sm.handleResponse(resp); err != nil {
				return fmt.Errorf("failed to handle RPC response: %w", err)
			}
		}

		// Handle error message
		errMsg := msg.GetError()
		if errMsg != nil {
			sm.handleError(errMsg)
		}
	}
}

// waitForResponse waits for an RPC response with the given request ID.
func (sm *StreamManager) waitForResponse(ctx context.Context, requestID string) (interface{}, error) {
	// Create channel for this request
	respChan := make(chan interface{}, 1)

	sm.requestsMu.Lock()
	sm.pendingCalls[requestID] = respChan
	sm.requestsMu.Unlock()

	defer func() {
		sm.requestsMu.Lock()
		delete(sm.pendingCalls, requestID)
		sm.requestsMu.Unlock()
	}()

	select {
	case resp := <-respChan:
		if err, ok := resp.(error); ok {
			return nil, err
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// handleResponse processes an RPC response from the plugin.
func (sm *StreamManager) handleResponse(rpcResp *pluginframeworkv1.PluginRPCResponse) error {
	requestID := rpcResp.GetRequestId()

	sm.requestsMu.RLock()
	respChan, exists := sm.pendingCalls[requestID]
	sm.requestsMu.RUnlock()

	if !exists {
		return fmt.Errorf("no pending call for request ID: %s", requestID)
	}

	// Return raw bytes - caller is responsible for unmarshaling
	select {
	case respChan <- rpcResp.GetPayload():
		// Response sent to waiter
	default:
		// Channel full, cannot send response
	}

	return nil
}

// handleError processes an error message from the plugin.
func (sm *StreamManager) handleError(errMsg *pluginframeworkv1.PluginError) {
	// If we can identify which request this error is for, we should send it to that waiter
	// For now, we just log it
}

// generateRequestID generates a unique request ID.
func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
