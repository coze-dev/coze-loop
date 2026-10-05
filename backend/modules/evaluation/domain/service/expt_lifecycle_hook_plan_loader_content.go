// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func frozenPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneFrozenContent(value *entity.Content, path map[*entity.Content]bool, depth int) (*entity.Content, error) {
	if value == nil {
		return nil, nil
	}
	if depth > 128 || path[value] {
		return nil, entity.ErrHookFrozenContentUnavailable
	}
	path[value] = true
	defer delete(path, value)
	c := *value
	c.ContentType = frozenPointer(value.ContentType)
	c.Format = frozenPointer(value.Format)
	c.Text = frozenPointer(value.Text)
	c.ContentOmitted = frozenPointer(value.ContentOmitted)
	c.FullContentBytes = frozenPointer(value.FullContentBytes)
	if value.Image != nil {
		i := *value.Image
		i.Name = frozenPointer(i.Name)
		i.URL = frozenPointer(i.URL)
		i.URI = frozenPointer(i.URI)
		i.ThumbURL = frozenPointer(i.ThumbURL)
		i.StorageProvider = frozenPointer(i.StorageProvider)
		c.Image = &i
	}
	if value.Audio != nil {
		a := *value.Audio
		a.Format = frozenPointer(a.Format)
		a.Name = frozenPointer(a.Name)
		a.URL = frozenPointer(a.URL)
		a.URI = frozenPointer(a.URI)
		a.StorageProvider = frozenPointer(a.StorageProvider)
		c.Audio = &a
	}
	if value.Video != nil {
		v := *value.Video
		v.Name = frozenPointer(v.Name)
		v.URL = frozenPointer(v.URL)
		v.URI = frozenPointer(v.URI)
		v.ThumbURL = frozenPointer(v.ThumbURL)
		v.StorageProvider = frozenPointer(v.StorageProvider)
		c.Video = &v
	}
	if value.FullContent != nil {
		f := *value.FullContent
		f.Provider = frozenPointer(f.Provider)
		f.Name = frozenPointer(f.Name)
		f.URI = frozenPointer(f.URI)
		f.URL = frozenPointer(f.URL)
		f.ThumbURL = frozenPointer(f.ThumbURL)
		c.FullContent = &f
	}
	if value.MultiPart != nil {
		c.MultiPart = make([]*entity.Content, len(value.MultiPart))
		for i, part := range value.MultiPart {
			var err error
			c.MultiPart[i], err = cloneFrozenContent(part, path, depth+1)
			if err != nil {
				return nil, err
			}
		}
	}
	return &c, nil
}

func defaultFrozenContent(c *entity.Content, schema frozenFieldSchema) {
	if c == nil {
		return
	}
	if c.ContentType == nil || *c.ContentType == "" {
		c.ContentType = frozenPointer(&schema.kind)
	}
	if c.Format == nil || *c.Format == 0 {
		c.Format = frozenPointer(&schema.format)
	}
}

func frozenReference(value *string) bool {
	return value != nil && strings.TrimSpace(*value) != "" && utf8.ValidString(*value)
}

func completeFrozenContent(c *entity.Content) bool {
	if c == nil || c.ContentOmitted != nil && *c.ContentOmitted || c.Text != nil && !utf8.ValidString(*c.Text) {
		return false
	}
	switch c.GetContentType() {
	case entity.ContentTypeText:
		// GetText treats an unmarked nil text as empty; a full-content reference is not an empty value.
		if c.Text == nil && c.FullContent != nil {
			return false
		}
	case entity.ContentTypeImage:
		if c.Image == nil || !frozenReference(c.Image.URL) && !frozenReference(c.Image.URI) {
			return false
		}
	case entity.ContentTypeAudio:
		if c.Audio == nil || !frozenReference(c.Audio.URL) && !frozenReference(c.Audio.URI) {
			return false
		}
	case entity.ContentTypeVideo:
		if c.Video == nil || !frozenReference(c.Video.URL) && !frozenReference(c.Video.URI) {
			return false
		}
	case entity.ContentTypeMultipart, entity.ContentTypeMultipartVariable:
	default:
		return false
	}
	for _, part := range c.MultiPart {
		if !completeFrozenContent(part) {
			return false
		}
	}
	return true
}

// This validation is limited to message_list completion. Part keys are unavailable here, so unresolved references cannot be guessed.
func completeResolvedFrozenMessageList(c *entity.Content) bool {
	if c == nil || c.Text == nil {
		return false
	}
	body := bytes.TrimSpace([]byte(*c.Text))
	if len(body) == 0 || body[0] != '[' {
		return false
	}
	var messages []map[string]json.RawMessage
	if json.Unmarshal(body, &messages) != nil {
		return false
	}
	for _, message := range messages {
		if message == nil {
			return false
		}
		content, ok := message["content"]
		if !ok {
			continue
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil || unresolvedFrozenAttachment(value) {
			return false
		}
	}
	return true
}

func unresolvedFrozenAttachment(value any) bool {
	switch v := value.(type) {
	case string:
		const prefix = "<attachment#"
		for {
			start := strings.Index(v, prefix)
			if start < 0 {
				return false
			}
			v = v[start+len(prefix):]
			end := strings.IndexByte(v, '>')
			if end > 0 && !strings.ContainsRune(v[:end], '<') {
				return true
			}
		}
	case []any:
		for _, element := range v {
			if unresolvedFrozenAttachment(element) {
				return true
			}
		}
	case map[string]any:
		for _, element := range v {
			if unresolvedFrozenAttachment(element) {
				return true
			}
		}
	}
	return false
}

func cloneFrozenUser(u *entity.UserInfo) *entity.UserInfo {
	if u == nil {
		return nil
	}
	out := *u
	out.Name = frozenPointer(u.Name)
	out.EnName = frozenPointer(u.EnName)
	out.AvatarURL = frozenPointer(u.AvatarURL)
	out.AvatarThumb = frozenPointer(u.AvatarThumb)
	out.OpenID = frozenPointer(u.OpenID)
	out.UnionID = frozenPointer(u.UnionID)
	out.UserID = frozenPointer(u.UserID)
	out.Email = frozenPointer(u.Email)
	return &out
}

func cloneFrozenBase(b *entity.BaseInfo) *entity.BaseInfo {
	if b == nil {
		return nil
	}
	out := *b
	out.CreatedBy = cloneFrozenUser(b.CreatedBy)
	out.UpdatedBy = cloneFrozenUser(b.UpdatedBy)
	out.CreatedAt = frozenPointer(b.CreatedAt)
	out.UpdatedAt = frozenPointer(b.UpdatedAt)
	out.DeletedAt = frozenPointer(b.DeletedAt)
	return &out
}
