# Legacy application submission fixture

`legacy-app-serialized-payload.json` is the complete output of `JSON.stringify(getSubmitValues(formValues))` from the production frontend function at commit `61744cf899460f6a99c37deaf471c11b354a56c8`:

`widgets/business-pages/opensource/evaluate/src/pages/experiment/create/tools.ts#getSubmitValues`

It is a synthetic fresh-form fixture using the authorized test IDs, not a network capture. It is recorded before host `transformCreateValues` and API-client body-key filtering; the extra UI fields remain in the JSON. It intentionally contains no `run_mode_config`, `target_runtime_param`, `eval_set_source_type` or `eval_set_configs`.

SHA256: `ae9353f3fd5ddefc455ca6336fdac9aa7b897a22b1ca363821ddec67949ca7b9`.

The Go regression adapts only the IDL `api.js_conv` int64 fields, then invokes the real SubmitExperiment, CreateExperiment, DTO conversion and ExptManager. External authentication/resource reads are test doubles; a controlled error stops the test at application lookup.
