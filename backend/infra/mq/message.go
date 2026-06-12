// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mq

import (
	"time"
)

type Message struct {
	Topic         string
	Body          []byte
	Tag           string
	Keys          []string
	PartitionKey  string
	Properties    map[string]string
	DeferDuration time.Duration
}

func NewMessage(topic string, body []byte) *Message {
	return &Message{
		Topic: topic,
		Body:  body,
	}
}

func NewOrderlyMessage(topic, partitionKey string, body []byte) *Message {
	return NewMessage(topic, body).WithPartitionKey(partitionKey)
}

func NewDeferMessage(topic string, deferDuration time.Duration, body []byte) *Message {
	return NewMessage(topic, body).WithDeferDuration(deferDuration)
}

func (m *Message) WithTag(tag string) *Message {
	m.Tag = tag
	return m
}

func (m *Message) WithKeys(keys []string) *Message {
	m.Keys = keys
	return m
}

func (m *Message) WithPartitionKey(partitionKey string) *Message {
	m.PartitionKey = partitionKey
	return m
}

func (m *Message) WithProperties(properties map[string]string) *Message {
	m.Properties = properties
	return m
}

func (m *Message) WithDeferDuration(deferDuration time.Duration) *Message {
	m.DeferDuration = deferDuration
	return m
}

type MessageExt struct {
	Message
	MsgID string
}
