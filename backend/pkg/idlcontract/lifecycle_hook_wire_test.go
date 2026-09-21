// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package idlcontract

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

func wireRoundTrip(t *testing.T, from, to thrift.TStruct) {
	t.Helper()
	b, err := thrift.NewTSerializer().Write(context.Background(), from)
	if err != nil {
		t.Fatal(err)
	}
	if err := thrift.NewTDeserializer().Read(to, b); err != nil {
		t.Fatal(err)
	}
}

func TestHookJSONTextAndSPIWireShape(t *testing.T) {
	for _, text := range []string{
		`{}`,
		`{"count":3,"cleanup":true,"nested":{"name":"calendar","value":null},"ids":["42",2]}`,
		`{"integer":9007199254740993,"id":"9223372036854775807"}`,
		`{"text":"{\"not\":\"an object\"}","enabled":false}`,
	} {
		t.Run(text, func(t *testing.T) {
			config := &domain.HookConfig{ParametersJSON: &text}
			b, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			var httpConfig domain.HookConfig
			if err := json.Unmarshal(b, &httpConfig); err != nil {
				t.Fatal(err)
			}
			var rpcConfig domain.HookConfig
			wireRoundTrip(t, &httpConfig, &rpcConfig)
			if rpcConfig.GetParametersJSON() != text {
				t.Fatal("management JSON/Thrift changed object text")
			}
			var apiConfig openapi.HookConfig
			if err := json.Unmarshal(b, &apiConfig); err != nil || apiConfig.GetParametersJSON() != text {
				t.Fatalf("OpenAPI differs from domain config: %v", err)
			}
			var rpcAPI openapi.HookConfig
			wireRoundTrip(t, &apiConfig, &rpcAPI)
			if rpcAPI.GetParametersJSON() != text {
				t.Fatal("OpenAPI Thrift changed object text")
			}

			parameters := spi.HookJSONObject(rpcConfig.GetParametersJSON())
			var request spi.InvokeExperimentHookRequest
			wireRoundTrip(t, &spi.InvokeExperimentHookRequest{Parameters: &parameters}, &request)
			if request.GetParameters() != text {
				t.Fatal("SPI Thrift changed parameters")
			}
			// The transport owns object encoding; generated management DTOs must remain strings.
			outbound, err := json.Marshal(struct {
				Parameters json.RawMessage `json:"parameters"`
			}{json.RawMessage(request.GetParameters())})
			if err != nil {
				t.Fatal(err)
			}
			if string(outbound) != `{"parameters":`+text+`}` {
				t.Fatalf("double encoding or number precision loss: %s", outbound)
			}
		})
	}
}

func TestAbsentHookConfigPreservesOptionalValues(t *testing.T) {
	for _, input := range []string{`{}`, `{"parameters_json":null}`} {
		var config domain.HookConfig
		if err := json.Unmarshal([]byte(input), &config); err != nil {
			t.Fatal(err)
		}
		var output domain.HookConfig
		wireRoundTrip(t, &config, &output)
		if output.ParametersJSON != nil || output.Enabled != nil || output.Retry != nil || output.TimeoutSeconds != nil {
			t.Fatal("absent configuration acquired defaults")
		}
		b, err := json.Marshal(&output)
		if err != nil || string(b) != `{}` {
			t.Fatalf("unexpected legacy shape: %s %v", b, err)
		}
	}
	var config domain.HookConfig
	if err := json.Unmarshal([]byte(`{"parameters_json":{"cleanup":true}}`), &config); err == nil {
		t.Fatal("object must not decode as a management string")
	}
}

func TestHookResponseUsesOnlyBusinessFields(t *testing.T) {
	status := spi.HookResultStatusSucceeded
	response := &spi.InvokeExperimentHookResponse{Status: &status, Result_: map[string]string{"action": "skipped", "json": `{"x":1}`}}
	var decoded spi.InvokeExperimentHookResponse
	wireRoundTrip(t, response, &decoded)
	b, err := json.Marshal(&decoded)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(b, &object); err != nil {
		t.Fatal(err)
	}
	if len(object) != 2 || object["status"] == nil || object["result"] == nil {
		t.Fatalf("unexpected response envelope: %s", b)
	}
	if decoded.Result_["json"] != `{"x":1}` || decoded.Result_["action"] != "skipped" {
		t.Fatal("result values must stay strings")
	}
	if err := json.Unmarshal([]byte(`{"status":"succeeded","result":{"count":3}}`), &decoded); err == nil {
		t.Fatal("non-string result accepted")
	}
	if strings.Contains(string(b), "operation_id") {
		t.Fatal("transport identity leaked into response")
	}
}
