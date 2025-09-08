package rpc

import "context"

type INotifyRPCAdapter interface {
	SendLarkMessageCard(ctx context.Context, userID, cardID string, param map[string]string) error
}
