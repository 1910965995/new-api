package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const drainExhaustedContent = "The account behind this API key doesn't have enough credits. Please top up and try again."

func setDrainTestKeywords(t *testing.T) {
	t.Helper()
	previous := operation_setting.AutomaticDisableKeywords
	operation_setting.AutomaticDisableKeywords = []string{"doesn't have enough credits"}
	t.Cleanup(func() { operation_setting.AutomaticDisableKeywords = previous })
}

func newDrainTestContext(t *testing.T, mode constant.MultiKeyMode, body, contentType string, isStream bool) (*gin.Context, *httptest.ResponseRecorder, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{contentType}},
	}
	info := &relaycommon.RelayInfo{
		IsStream:    isStream,
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:        constant.ChannelTypeOpenAI,
			ChannelIsMultiKey:  true,
			ChannelMultiKeyMode: mode,
			UpstreamModelName:  "gpt-drain-test",
		},
	}
	return c, recorder, resp, info
}

func drainCompletionBody() string {
	return `{"id":"resp_1","object":"chat.completion","model":"gpt-drain-test","choices":[{"index":0,"message":{"role":"assistant","content":"` +
		drainExhaustedContent + `"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":98,"total_tokens":118}}`
}

func TestOpenaiHandlerDrainKeywordReturnsChannelError(t *testing.T) {
	setDrainTestKeywords(t)
	c, recorder, resp, info := newDrainTestContext(t, constant.MultiKeyModeDrain, drainCompletionBody(), "application/json", false)

	usage, apiErr := OpenaiHandler(c, info, resp)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, types.IsChannelError(apiErr))
	assert.Equal(t, types.ErrorCodeChannelDrainKeyExhausted, apiErr.GetErrorCode())
	assert.Empty(t, recorder.Body.String(), "matched response must not be forwarded to the client")
}

func TestOpenaiHandlerNonDrainModeForwardsKeywordContent(t *testing.T) {
	setDrainTestKeywords(t)
	c, recorder, resp, info := newDrainTestContext(t, constant.MultiKeyModeRandom, drainCompletionBody(), "application/json", false)

	usage, apiErr := OpenaiHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 20, usage.PromptTokens)
	assert.Contains(t, recorder.Body.String(), "doesn't have enough credits")
}

func TestOaiStreamHandlerDrainKeywordStopsBeforeForwarding(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	setDrainTestKeywords(t)

	sse := strings.Join([]string{
		"data: " + `{"id":"resp_1","object":"chat.completion.chunk","model":"gpt-drain-test","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		"data: " + `{"id":"resp_1","object":"chat.completion.chunk","model":"gpt-drain-test","choices":[{"index":0,"delta":{"content":"` + drainExhaustedContent + `"},"finish_reason":null}]}`,
		"data: " + `{"id":"resp_1","object":"chat.completion.chunk","model":"gpt-drain-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"data: [DONE]",
	}, "\n\n") + "\n\n"

	c, recorder, resp, info := newDrainTestContext(t, constant.MultiKeyModeDrain, sse, "text/event-stream", true)

	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, types.IsChannelError(apiErr))
	assert.Equal(t, types.ErrorCodeChannelDrainKeyExhausted, apiErr.GetErrorCode())
	assert.NotContains(t, recorder.Body.String(), "doesn't have enough credits",
		"the chunk completing the keyword must be stopped before it reaches the client")
}
