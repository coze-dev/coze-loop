These synthetic fixtures were emitted by the frontend's real normalize/copy →
getSubmitValues → withSharedEvalSet → JSON.stringify path at commit
9e51cb07a879650abf7d55673729bd9818aa8abd. The manifest records their exact hashes.
No binary_version was added to fresh.json by the backend test.

The test emulates only the IDL api.js_conv int64 adaptation before decoding the
real CreateExperimentRequest, then calls ConvertCreateReq and CreateExpt up to
the mocked system target creation boundary. This is an offline cross-repository
contract test, not an AGW request, database write, or live PPE experiment.
