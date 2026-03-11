package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newResponsesStreamTest(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *http.Response) {
	t.Helper()
	oldStreamingTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.4",
		},
	}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	return ctx, recorder, info, resp
}

func TestOaiResponsesStreamHandler_PreWriteResponseFailedReturnsRetryableError(t *testing.T) {
	t.Parallel()

	body := "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"type\":\"server_error\",\"message\":\"boom\"}}}\n"
	ctx, recorder, info, resp := newResponsesStreamTest(t, body)

	usage, err := OaiResponsesStreamHandler(ctx, info, resp)
	require.Nil(t, usage)
	require.Error(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.Contains(t, err.Error(), "boom")
	require.Empty(t, recorder.Body.String())
}

func TestOaiResponsesStreamHandler_SilentEOFBeforeAnyEventReturnsRetryableError(t *testing.T) {
	t.Parallel()

	ctx, recorder, info, resp := newResponsesStreamTest(t, "")

	usage, err := OaiResponsesStreamHandler(ctx, info, resp)
	require.Nil(t, usage)
	require.Error(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
	require.Contains(t, err.Error(), "before any downstream event")
	require.Empty(t, recorder.Body.String())
}

func TestOaiResponsesStreamHandler_PostWriteFailureKeepsPassthrough(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"status\":\"in_progress\"}}\n",
		"data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"type\":\"server_error\",\"message\":\"boom\"}}}\n",
	}, "")
	ctx, recorder, info, resp := newResponsesStreamTest(t, body)

	usage, err := OaiResponsesStreamHandler(ctx, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Zero(t, usage.TotalTokens)
	require.Contains(t, recorder.Body.String(), "event: response.created")
	require.Contains(t, recorder.Body.String(), "event: response.failed")
}

func TestOaiResponsesStreamHandler_CompletedStreamKeepsUsage(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n",
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n",
		"data: [DONE]\n",
	}, "")
	ctx, recorder, info, resp := newResponsesStreamTest(t, body)

	usage, err := OaiResponsesStreamHandler(ctx, info, resp)
	require.Nil(t, err)
	require.Equal(t, &dto.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}, usage)
	require.Contains(t, recorder.Body.String(), "event: response.completed")
}
