package model_config

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Response bodies recorded from litellm/litellm:v1.82.0-stable.
const (
	recordedAddOK       = `{"model_id":"11111111-1111-4111-8111-111111111111","model_name":"ace-test-1","litellm_params":{"model":"21S7-zWLZ","api_key":"IbLRu0He","api_base":"fqDgHBfU"}}`
	recordedDuplicateID = `{"error":{"message":"{'error': 'Failed to add model to db. Check your server logs for more details.'}","type":"auth_error","param":"None","code":"500"}}`
	recordedNoDB        = `{"error":{"message":"{'error': \"No DB Connected. Here's how to do it - https://docs.litellm.ai/docs/proxy/virtual_keys\"}","type":"auth_error","param":"None","code":"500"}}`
	recordedBadMaster   = `{"error":{"message":"Authentication Error, Invalid proxy server token passed. Received API Key = sk-...rong, Key Hash (Token) =2ab06cfadcdcc1bb1a1ff9edefb028084a837b3ed7b95f97e602b960ee127471. Unable to find token in cache or LiteLLM_VerificationTokenTable","type":"token_not_found_in_db","param":"None","code":"401"}}`
	recordedPatchAbsent = `{"error":{"message":"Model 99999999-9999-4999-8999-999999999999 not found on proxy.","type":"not_found_error","param":null,"code":"404"}}`
	recordedDelAbsent   = `{"error":{"message":"{'error': 'Model with id=11111111-1111-4111-8111-111111111111 not found in db'}","type":"auth_error","param":"None","code":"400"}}`
	recordedHealthNoID  = `{"detail":{"error":"Model with ID nope not found"}}`
	recordedTestOK      = `{"status":"success","result":{"model":"openai/pre-save","api_base":"http://upstream:8080/v1","max_tokens":1}}`
	recordedTestFailed  = `{"status":"error","result":{"model":"openai/x","api_base":"http://nowhere:9/v1","error":"litellm.InternalServerError: InternalServerError: OpenAIException - Connection error.\nstack trace: Traceback (most recent call last):\n  File \"/usr/lib/python3.13/site-packages/aiohttp/connector.py\", line 1562"}}`
	recordedHealthy     = `{"healthy_endpoints":[{"api_base":"http://upstream:8080/v1","model":"openai/ace-test-1"}],"unhealthy_endpoints":[],"healthy_count":1,"unhealthy_count":0}`
	recordedUnhealthy   = `{"healthy_endpoints":[],"unhealthy_endpoints":[{"model":"openai/x","error":"litellm.AuthenticationError: Incorrect API key provided: sk-user-secret-9999\nstack trace: Traceback"}],"healthy_count":0,"unhealthy_count":1}`
	recordedModelInfo   = `{"data":[{"model_name":"static-fake","litellm_params":{"model":"openai/static-fake"},"model_info":{"id":"6bdfc587ee72ef87","db_model":false}},{"model_name":"ace-test-1","litellm_params":{"model":"openai/ace-test-1"},"model_info":{"id":"11111111-1111-4111-8111-111111111111","db_model":true}}]}`
	recordedReady       = `{"status":"connected","db":"connected","cache":null,"litellm_version":"1.82.0"}`
	recordedReadyNoDB   = `{"status":"healthy","db":"Not connected","cache":null,"litellm_version":"1.82.0"}`
	// Tests of an Azure OpenAI GPT-5.1 deployment (2026-10-28): as azure/<deployment>
	// it refuses max_tokens; as azure/gpt5_series/<deployment> it cannot answer
	// in one token. The same deployment with a wrong name or key fails.
	recordedTestTokenCap      = `{"status":"error","result":{"model":"azure/gpt4o","api_base":"https://acme.openai.azure.com","api_version":"2024-12-01-preview","max_tokens":1,"error":"litellm.BadRequestError: AzureException BadRequestError - Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.\nstack trace: Traceback (most recent call last):\n  File \"/app/litellm/main.py\", line 613, in acompletion"}}`
	recordedTestOutputCap     = `{"status":"error","result":{"model":"azure/gpt5_series/gpt4o","api_base":"https://acme.openai.azure.com","api_version":"2024-12-01-preview","max_tokens":1,"error":"litellm.BadRequestError: AzureException BadRequestError - Could not finish the message because max_tokens or model output limit was reached. Please try again with higher max_tokens.\nstack trace: Traceback (most recent call last):\n  File \"/app/litellm/main.py\", line 613, in acompletion"}}`
	recordedTestNoDeployment  = `{"status":"error","result":{"model":"azure/admin","api_base":"https://acme.openai.azure.com","max_tokens":1,"error":"litellm.NotFoundError: AzureException NotFoundError - The API deployment for this resource does not exist. If you created the deployment within the last 5 minutes, please wait a moment and try again.\nstack trace: Traceback"}}`
	recordedTestBadKey        = `{"status":"error","result":{"model":"azure/gpt4o","api_base":"https://acme.openai.azure.com","max_tokens":1,"error":"litellm.AuthenticationError: AzureException AuthenticationError - Access denied due to invalid subscription key or wrong API endpoint. Make sure to provide a valid key for an active subscription and use a correct regional API endpoint for your resource.\nstack trace: Traceback"}}`
	recordedUnhealthyTokenCap = `{"healthy_endpoints":[],"unhealthy_endpoints":[{"model":"azure/gpt4o","error":"litellm.BadRequestError: AzureException BadRequestError - Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.\nstack trace: Traceback"}],"healthy_count":0,"unhealthy_count":1}`
)

const testMasterKey = "sk-master-test-0001"

type recordedRequest struct {
	method, path, auth string
	body               map[string]interface{}
}

type reply struct {
	status int
	body   string
}

// fakeLiteLLM answers each "METHOD /path" with a queue of replies, the last
// one repeating, and records every request.
type fakeLiteLLM struct {
	mu       sync.Mutex
	replies  map[string][]reply
	requests []recordedRequest
	delay    time.Duration
}

func newFakeLiteLLM(t *testing.T, replies map[string][]reply) (*fakeLiteLLM, *litellmClient) {
	fake := &fakeLiteLLM{replies: replies}
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(server.Close)
	client := NewLiteLLMClient(server.URL+"/", testMasterKey).(*litellmClient)
	return fake, client
}

func (f *fakeLiteLLM) serve(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	request := recordedRequest{method: r.Method, path: r.URL.RequestURI(), auth: r.Header.Get("Authorization")}
	_ = json.Unmarshal(data, &request.body)
	f.mu.Lock()
	f.requests = append(f.requests, request)
	key := r.Method + " " + r.URL.Path
	queue := f.replies[key]
	answer := reply{status: http.StatusNotFound, body: `{"detail":"Not Found"}`}
	if len(queue) > 0 {
		answer = queue[0]
		if len(queue) > 1 {
			f.replies[key] = queue[1:]
		}
	}
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	w.WriteHeader(answer.status)
	_, _ = w.Write([]byte(answer.body))
}

func (f *fakeLiteLLM) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

var testDeployment = Deployment{ModelID: "11111111-1111-4111-8111-111111111111", Alias: "ace-test-1", Model: "azure/gpt-4o", APIBase: "https://acme.openai.azure.com", APIVersion: "2024-08-01-preview"}

func TestAddModelSendsTheModelIDAndKeyWithTheMasterKey(t *testing.T) {
	fake, client := newFakeLiteLLM(t, map[string][]reply{"POST /model/new": {{200, recordedAddOK}}})

	if err := client.AddModel(context.Background(), testDeployment, "az-user-key"); err != nil {
		t.Fatalf("AddModel() error = %v", err)
	}
	got := fake.requests[0]
	if got.auth != "Bearer "+testMasterKey {
		t.Fatalf("Authorization = %q", got.auth)
	}
	params := got.body["litellm_params"].(map[string]interface{})
	info := got.body["model_info"].(map[string]interface{})
	if got.body["model_name"] != "ace-test-1" || info["id"] != testDeployment.ModelID ||
		params["model"] != "azure/gpt-4o" || params["api_base"] != testDeployment.APIBase ||
		params["api_version"] != "2024-08-01-preview" || params["api_key"] != "az-user-key" {
		t.Fatalf("request body = %v", got.body)
	}
}

func TestUpdateAndDeleteAddressTheModelByID(t *testing.T) {
	fake, client := newFakeLiteLLM(t, map[string][]reply{
		"PATCH /model/" + testDeployment.ModelID + "/update": {{200, `{}`}},
		"POST /model/delete": {{200, `{"message":"deleted"}`}},
	})
	if err := client.UpdateModel(context.Background(), testDeployment, "new-key"); err != nil {
		t.Fatalf("UpdateModel() error = %v", err)
	}
	if err := client.DeleteModel(context.Background(), testDeployment.ModelID); err != nil {
		t.Fatalf("DeleteModel() error = %v", err)
	}
	patch := fake.requests[0].body["litellm_params"].(map[string]interface{})
	if patch["api_key"] != "new-key" || patch["model"] != "azure/gpt-4o" {
		t.Fatalf("the PATCH must carry every parameter and the new key, got %v", patch)
	}
	if fake.requests[1].body["id"] != testDeployment.ModelID {
		t.Fatalf("delete body = %v", fake.requests[1].body)
	}
}

func TestLiteLLMAnswersAreClassified(t *testing.T) {
	cases := []struct {
		name   string
		call   func(*litellmClient) error
		route  string
		answer reply
		want   error
	}{
		{"no database is the registry being off, not an unknown outcome", addCall, "POST /model/new", reply{500, recordedNoDB}, ErrRegistryUnavailable},
		{"store_model_in_db unset", addCall, "POST /model/new", reply{500, `{"error":{"message":"{'error': \"Set ` + "`'STORE_MODEL_IN_DB='True'`" + ` in your env to enable this feature.\"}","code":"500"}}`}, ErrRegistryUnavailable},
		{"a failed database write is a known failure", addCall, "POST /model/new", reply{500, recordedDuplicateID}, ErrRejected},
		{"a wrong master key", addCall, "POST /model/new", reply{401, recordedBadMaster}, ErrUnauthorized},
		{"other 5xx: the outcome is unknown", addCall, "POST /model/new", reply{502, `Bad Gateway`}, ErrUnreachable},
		{"a 4xx: rejected", addCall, "POST /model/new", reply{422, `{"detail":[{"msg":"field required"}]}`}, ErrRejected},
		{"PATCH of an unknown ID", updateCall, "PATCH /model/" + testDeployment.ModelID + "/update", reply{404, recordedPatchAbsent}, ErrNotFound},
		{"delete of an unknown ID", deleteCall, "POST /model/delete", reply{400, recordedDelAbsent}, ErrNotFound},
		{"delete without a database", deleteCall, "POST /model/delete", reply{500, recordedNoDB}, ErrRegistryUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, client := newFakeLiteLLM(t, map[string][]reply{tc.route: {tc.answer}})
			err := tc.call(client)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if strings.Contains(err.Error(), "rong") || strings.Contains(err.Error(), testMasterKey) {
				t.Fatalf("the error leaks key material: %v", err)
			}
		})
	}
}

func addCall(c *litellmClient) error { return c.AddModel(context.Background(), testDeployment, "k") }
func updateCall(c *litellmClient) error {
	return c.UpdateModel(context.Background(), testDeployment, "k")
}
func deleteCall(c *litellmClient) error {
	return c.DeleteModel(context.Background(), testDeployment.ModelID)
}

func TestWritesAreNotRetriedButReadsAreOnce(t *testing.T) {
	fake, client := newFakeLiteLLM(t, map[string][]reply{
		"POST /model/new":    {{503, `unavailable`}},
		"GET /v1/model/info": {{503, `unavailable`}, {200, recordedModelInfo}},
	})
	if err := client.AddModel(context.Background(), testDeployment, "k"); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("AddModel() error = %v, want ErrUnreachable", err)
	}
	if fake.count() != 1 {
		t.Fatalf("a write was sent %d times, want once", fake.count())
	}
	models, err := client.ListModels(context.Background())
	if err != nil || len(models) != 2 {
		t.Fatalf("ListModels() = %v, %v; want the second attempt's two models", models, err)
	}
	if models[0] != (RemoteModel{Alias: "static-fake", ModelID: "6bdfc587ee72ef87"}) ||
		models[1] != (RemoteModel{Alias: "ace-test-1", ModelID: testDeployment.ModelID, DBModel: true}) {
		t.Fatalf("ListModels() = %+v", models)
	}
}

func TestAnUnreachableGatewayIsAnUnknownOutcome(t *testing.T) {
	client := NewLiteLLMClient("http://127.0.0.1:1", testMasterKey)
	if err := client.AddModel(context.Background(), testDeployment, "k"); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("AddModel() error = %v, want ErrUnreachable", err)
	}
}

func TestAnEmptyKeyIsNeverSent(t *testing.T) {
	fake, client := newFakeLiteLLM(t, map[string][]reply{"POST /model/new": {{200, recordedAddOK}}})
	if err := client.AddModel(context.Background(), testDeployment, " "); !errors.Is(err, ErrRejected) {
		t.Fatalf("AddModel() error = %v, want ErrRejected", err)
	}
	if _, err := client.TestConnection(context.Background(), testDeployment, ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("TestConnection() error = %v, want ErrRejected", err)
	}
	if fake.count() != 0 {
		t.Fatal("a request with an empty key reached LiteLLM")
	}
}

func TestConnectionTests(t *testing.T) {
	_, client := newFakeLiteLLM(t, map[string][]reply{"POST /health/test_connection": {{200, recordedTestOK}, {200, recordedTestFailed}, {500, recordedNoDB}}})

	ok, err := client.TestConnection(context.Background(), testDeployment, "k")
	if err != nil || !ok.OK {
		t.Fatalf("a successful test = %+v, %v", ok, err)
	}
	failed, err := client.TestConnection(context.Background(), testDeployment, "k")
	if err != nil || failed.OK || failed.Error != "litellm.InternalServerError: InternalServerError: OpenAIException - Connection error." {
		t.Fatalf("a failed test = %+v, %v; want the error without its stack trace", failed, err)
	}
	if _, err := client.TestConnection(context.Background(), testDeployment, "k"); !errors.Is(err, ErrRegistryUnavailable) {
		t.Fatalf("a test without a database: error = %v", err)
	}
}

func TestAReasoningModelRefusingTheTestTokenCapPasses(t *testing.T) {
	_, client := newFakeLiteLLM(t, map[string][]reply{
		"POST /health/test_connection": {{200, recordedTestTokenCap}, {200, recordedTestOutputCap}, {200, recordedTestNoDeployment}, {200, recordedTestBadKey}},
		"GET /health":                  {{200, recordedUnhealthyTokenCap}},
	})

	for _, name := range []string{"refused max_tokens", "ran out of its one token"} {
		if result, err := client.TestConnection(context.Background(), testDeployment, "k"); err != nil || !result.OK || result.Error != "" {
			t.Fatalf("a model that %s: TestConnection() = %+v, %v; want a pass", name, result, err)
		}
	}
	for _, want := range []string{"The API deployment for this resource does not exist", "Access denied due to invalid subscription key"} {
		result, err := client.TestConnection(context.Background(), testDeployment, "k")
		if err != nil || result.OK || !strings.Contains(result.Error, want) {
			t.Fatalf("TestConnection() = %+v, %v; want a failure naming %q", result, err, want)
		}
	}
	if result, err := client.HealthCheck(context.Background(), testDeployment.ModelID); err != nil || !result.OK {
		t.Fatalf("a saved model that refused max_tokens: HealthCheck() = %+v, %v; want a pass", result, err)
	}
}

func TestASlowModelIsAFailedTestNotAnError(t *testing.T) {
	fake, client := newFakeLiteLLM(t, map[string][]reply{"POST /health/test_connection": {{200, recordedTestOK}}})
	fake.delay, client.testTimeout = 200*time.Millisecond, 20*time.Millisecond

	result, err := client.TestConnection(context.Background(), testDeployment, "k")
	if err != nil || result.OK || !strings.Contains(result.Error, "no answer within") {
		t.Fatalf("TestConnection() = %+v, %v; want a timed-out result", result, err)
	}
}

func TestHealthChecks(t *testing.T) {
	_, client := newFakeLiteLLM(t, map[string][]reply{"GET /health": {{200, recordedHealthy}, {200, recordedUnhealthy}, {404, recordedHealthNoID}}})

	healthy, err := client.HealthCheck(context.Background(), testDeployment.ModelID)
	if err != nil || !healthy.OK {
		t.Fatalf("healthy = %+v, %v", healthy, err)
	}
	unhealthy, err := client.HealthCheck(context.Background(), testDeployment.ModelID)
	if err != nil || unhealthy.OK || !strings.HasPrefix(unhealthy.Error, "litellm.AuthenticationError") || strings.Contains(unhealthy.Error, "stack trace") {
		t.Fatalf("unhealthy = %+v, %v", unhealthy, err)
	}
	if _, err := client.HealthCheck(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown model: error = %v, want ErrNotFound", err)
	}
}

func TestRegistryConnected(t *testing.T) {
	_, client := newFakeLiteLLM(t, map[string][]reply{"GET /health/readiness": {{200, recordedReady}, {200, recordedReadyNoDB}}})
	for _, want := range []bool{true, false} {
		if got, err := client.RegistryConnected(context.Background()); err != nil || got != want {
			t.Fatalf("RegistryConnected() = %v, %v; want %v", got, err, want)
		}
	}
}

func TestCleanRedactsKeysAndBoundsTheMessage(t *testing.T) {
	client := NewLiteLLMClient("http://x", testMasterKey).(*litellmClient)
	got := client.clean("bad key sk-user-secret-9999 and master "+testMasterKey+strings.Repeat("x", 600), "sk-user-secret-9999")
	if strings.Contains(got, "sk-user-secret-9999") || strings.Contains(got, testMasterKey) || len(got) > maxErrorLength+3 {
		t.Fatalf("clean() = %q", got)
	}
}

func TestAdminURL(t *testing.T) {
	cases := []struct{ admin, openAI, want string }{
		{"", "http://litellm.ace.svc.cluster.local:14000/v1", "http://litellm.ace.svc.cluster.local:14000"},
		{"", "http://litellm:14000/v1/", "http://litellm:14000"},
		{"http://litellm:4000/", "http://172.26.0.1:14000/v1", "http://litellm:4000"},
	}
	for _, tc := range cases {
		if got := AdminURL(tc.admin, tc.openAI); got != tc.want {
			t.Errorf("AdminURL(%q, %q) = %q, want %q", tc.admin, tc.openAI, got, tc.want)
		}
	}
}
