// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package idlcontract

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/cloudwego/thriftgo/parser"
	"github.com/cloudwego/thriftgo/semantic"
)

var files = []string{"domain/expt.thrift", "domain_openapi/experiment.thrift", "coze.loop.evaluation.expt.thrift", "coze.loop.evaluation.openapi.thrift", "coze.loop.evaluation.spi.thrift"}

func readIDL(t *testing.T, name string) *parser.Thrift {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	p := filepath.Join(filepath.Dir(file), "../../../idl/thrift/coze/loop/evaluation", name)
	a, err := parser.ParseFile(p, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func digest(v any) string { b, _ := json.Marshal(v); return fmt.Sprintf("%x", sha256.Sum256(b)) }

func snapshot(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, file := range files {
		a := readIDL(t, file)
		for _, s := range a.Structs {
			for _, f := range s.Fields {
				copy := *f
				copy.ReservedComments = ""
				out[file+"/field/"+s.Name+"/"+fmt.Sprint(f.ID)] = digest(copy)
			}
		}
		for _, s := range a.Services {
			for _, f := range s.Functions {
				copy := *f
				copy.ReservedComments = ""
				out[file+"/method/"+s.Name+"/"+f.Name] = digest(copy)
			}
		}
	}
	return out
}

func TestLegacyIDLCompatibility(t *testing.T) {
	b, err := os.ReadFile("testdata/legacy_contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var old map[string]string
	if err := json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	current := snapshot(t)
	for key, hash := range old {
		if current[key] != hash {
			t.Errorf("legacy field/method changed or removed: %s", key)
		}
	}
	for _, file := range files {
		a := readIDL(t, file)
		for _, s := range a.Structs {
			for _, f := range s.Fields {
				key := file + "/field/" + s.Name + "/" + fmt.Sprint(f.ID)
				if _, exists := old[key]; !exists && (f.Requiredness != parser.FieldType_Optional || f.Default != nil) {
					t.Errorf("new field must preserve absent-value semantics: %s", key)
				}
			}
		}
	}
	t.Logf("protected %d legacy field/method contracts", len(old))
}

func requireFields(t *testing.T, a *parser.Thrift, name string, want []string) *parser.StructLike {
	t.Helper()
	for _, s := range a.Structs {
		if s.Name == name {
			if len(s.Fields) != len(want) {
				t.Fatalf("%s: got %d fields, want %d", name, len(s.Fields), len(want))
			}
			for i, expected := range want {
				f := s.Fields[i]
				id := int32(i + 1)
				if expected == "Base" {
					id = 255
				}
				if f.ID != id || f.Name != expected || f.Requiredness != parser.FieldType_Optional {
					t.Errorf("%s field %d: wrong ID/name/requiredness", name, i+1)
				}
			}
			return s
		}
	}
	t.Fatalf("missing contract %s", name)
	return nil
}

func TestLifecycleHookContract(t *testing.T) {
	for _, file := range files[:2] {
		t.Run(file, func(t *testing.T) {
			a := readIDL(t, file)
			cfg := requireFields(t, a, "HookConfig", []string{"enabled", "access_protocol", "invoke_http_info", "environment", "lane", "parameters_json", "timeout_seconds", "retry", "on_failure"})
			if cfg.Fields[5].Type.Name != "string" {
				t.Fatal("management parameters_json must be string")
			}
			for _, f := range cfg.Fields {
				for _, ann := range f.Annotations {
					if ann.Key == "api.body" {
						t.Fatal("nested config must not use api.body")
					}
				}
			}
		})
	}
	a := readIDL(t, files[4])
	req := requireFields(t, a, "InvokeExperimentHookRequest", []string{"schema_version", "event_type", "operation_id", "idempotency_key", "delivery_id", "attempt", "occurred_at", "context", "parameters"})
	if req.Fields[8].Type.Name != "HookJSONObject" {
		t.Fatal("SPI parameters must use JSON object typedef")
	}
	resp := requireFields(t, a, "InvokeExperimentHookResponse", []string{"status", "result", "error"})
	m := resp.Fields[1].Type
	if m.Name != "map" || m.KeyType.Name != "string" || m.ValueType.Name != "string" {
		t.Fatal("SPI result must be map<string,string>")
	}
	found := false
	for _, td := range a.Typedefs {
		if td.Alias == "HookJSONObject" {
			found = td.Type.Name == "string"
		}
	}
	if !found {
		t.Fatal("missing string HookJSONObject alias")
	}
	cb := requireFields(t, readIDL(t, files[2]), "SubmitScheduledExptFromTemplateRequest", []string{"workspace_id", "template_id", "binding_id", "binding_version", "Base"})
	_ = cb
}

func TestHookConfigurationEntryPoints(t *testing.T) {
	checks := map[string]map[string]int32{
		files[0]: {"Experiment": 120, "ExptTemplate": 12},
		files[1]: {"Experiment": 120, "ExptTemplate": 12},
		files[2]: {"CreateExperimentRequest": 111, "SubmitExperimentRequest": 111, "UpdateExperimentRequest": 111, "CreateExperimentTemplateRequest": 41, "UpdateExperimentTemplateRequest": 41, "SubmitExptFromTemplateRequest": 11},
		files[3]: {"SubmitExperimentOApiRequest": 53, "CreateExptTemplateOApiRequest": 31, "UpdateExptTemplateOApiRequest": 31, "SubmitExptFromTemplateOApiRequest": 11},
	}
	for file, structs := range checks {
		for name, id := range structs {
			found := false
			for _, s := range readIDL(t, file).Structs {
				if s.Name == name {
					for _, f := range s.Fields {
						if f.ID == id && f.Name == "lifecycle_hook_conf" && f.Requiredness == parser.FieldType_Optional {
							found = true
						}
					}
				}
			}
			if !found {
				t.Errorf("missing optional lifecycle config: %s %s field %d", file, name, id)
			}
		}
	}
}

func TestIncludesAndHTTPMappings(t *testing.T) {
	for _, file := range files {
		a := readIDL(t, file)
		if err := semantic.ResolveSymbols(a); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		seen := map[string]string{}
		for _, s := range a.Services {
			for _, fn := range s.Functions {
				if strings.Contains(fn.Name, "HTTPBridge") {
					t.Errorf("unapproved bridge method: %s", fn.Name)
				}
				for _, ann := range fn.Annotations {
					if ann.Key == "api.get" || ann.Key == "api.post" || ann.Key == "api.patch" || ann.Key == "api.put" || ann.Key == "api.delete" {
						key := ann.Key + ":" + strings.Join(ann.Values, ",")
						if prev := seen[key]; prev != "" {
							t.Errorf("duplicate HTTP mapping %s: %s/%s", key, prev, fn.Name)
						}
						seen[key] = fn.Name
						if fn.Name == "SubmitScheduledExptFromTemplate" {
							t.Fatal("scheduled callback must remain internal")
						}
					}
				}
			}
		}
	}
}

func TestScheduledCallbackDoesNotExtendLegacyService(t *testing.T) {
	ast := readIDL(t, files[2])
	found := false
	for _, service := range ast.Services {
		for _, method := range service.Functions {
			if method.Name != "SubmitScheduledExptFromTemplate" {
				continue
			}
			if service.Name != "ExperimentScheduleService" || len(method.Annotations) != 0 {
				t.Fatal("scheduled callback must be an independent, internal service")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing internal scheduled callback")
	}
}

func TestLegacySPIServiceMethodSet(t *testing.T) {
	want := []string{"AsyncInvokeEvalTarget", "AsyncInvokeEvaluator", "InvokeEvalTarget", "InvokeEvaluator", "SearchEvalTarget"}
	for _, service := range readIDL(t, files[4]).Services {
		if service.Name != "EvaluationSPIService" {
			continue
		}
		var got []string
		for _, method := range service.Functions {
			got = append(got, method.Name)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("legacy implementer contract changed: got %v, want %v", got, want)
		}
		return
	}
	t.Fatal("missing legacy EvaluationSPIService")
}

func TestHookSPIHasIndependentService(t *testing.T) {
	for _, service := range readIDL(t, files[4]).Services {
		if service.Name != "ExperimentHookSPIService" {
			continue
		}
		if len(service.Functions) != 1 || service.Functions[0].Name != "InvokeExperimentHook" || len(service.Functions[0].Annotations) != 0 {
			t.Fatal("Hook SPI must be a separate logical interface without HTTP routes")
		}
		return
	}
	t.Fatal("missing independent ExperimentHookSPIService")
}
