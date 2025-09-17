package rpc

import "context"

//go:generate mockgen -destination=mocks/notify.go -package=mocks . INotifyRPCAdapter
type INotifyRPCAdapter interface {
	SendLarkMessageCard(ctx context.Context, userID, cardID string, param map[string]string) error
}
