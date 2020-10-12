package lambda

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

const (
	// DecryptionPasswordStageVar is the name of API gateway stage variable that feeds laitos program data decryption password.
	DecryptionPasswordStageVar = "LAITOS_PROGRAM_DATA_DECRYPTION_PASSWORD"
	// UpstreamWebServerPort is the port number where lambda handler expects to find laitos web server.
	UpstreamWebServerPort = 52535
)

/*
IsProgramDataDecrypted is set to true after an attempt has been made to decrypt laitos program data using password fed through
API gateway stage variable DecryptionPasswordStageVar.
*/
var IsProgramDataDecrypted bool

/*
Handler is an AWS Lambda handler function that looks for incoming HTTP requests from lambda invocation event data,
proxies them to laitos web server, and sends the responses back to lambda.
It behaves similar to one of the laitos daemon programs, and uses a similar convention in its interface.
*/
type Handler struct {
	logger lalog.Logger
}

/*
StartAndBlock continuously handles lambda invocations, each of which represents an HTTP request coming from API
gateway intended for the laitos web server. The function blocks caller indefinitely.
*/
func (hand *Handler) StartAndBlock() {
	hand.logger = lalog.Logger{
		ComponentName: "lambda",
		ComponentID:   nil,
	}
	lambdaAPIHostNamePort := os.Getenv("AWS_LAMBDA_RUNTIME_API")
	lambdaTaskRoot := os.Getenv("LAMBDA_TASK_ROOT")
	lambdaHandlerName := os.Getenv("_HANDLER")
	// Log crucial environment input for diagnosis on CloudWatch log group
	hand.logger.Info("StartAndBlock", "", nil, "AWS_LAMBDA_RUNTIME_API is \"%s\", LAMBDA_TASK_ROOT is \"%s\", _HANDLER is \"%s\"",
		lambdaAPIHostNamePort, lambdaTaskRoot, lambdaHandlerName)
	for {
		hand.logger.Info("StartAndBlock", lambdaHandlerName, nil, "looking for the next lambda invocation")
		if err := hand.getAndProcessLambdaInvocation(lambdaAPIHostNamePort, UpstreamWebServerPort); err != nil {
			hand.logger.Warning("StartAndBlock", lambdaHandlerName, err, "failed to process invocation")
		}
	}
}

/*
getAndProcessLambdaInvocation looks for the next lambda function invocation that represents an HTTP request, decodes request
details, sends it to laitos web server, and eventually responds to the lambda invocation with a structure representing the
web server response.
*/
func (hand *Handler) getAndProcessLambdaInvocation(lambdaAPIHostNamePort string, webServerPort int) error {
	nextInvocResp, err := inet.DoHTTP(context.TODO(), inet.HTTPRequest{Method: http.MethodGet}, fmt.Sprintf("http://%s/2018-06-01/runtime/invocation/next", lambdaAPIHostNamePort))
	if err != nil {
		return err
	}
	if err := nextInvocResp.Non2xxToError(); err != nil {
		return err
	}
	// Lambda-Runtime-Aws-Request-Id comes from next invocation's response header, not to be confused with requestContext.requestId.
	awsRequestID := nextInvocResp.Header.Get("Lambda-Runtime-Aws-Request-Id")
	hand.logger.Info("processLambdaInvocation", lambdaAPIHostNamePort, nil,
		"request ID \"%s\" of %d bytes just arrived: %s", awsRequestID, len(nextInvocResp.Body), nextInvocResp.GetBodyUpTo(2000))
	invocResult, err := hand.decodeAndHandleHTTPRequest(awsRequestID, nextInvocResp.Body, webServerPort)
	if err != nil {
		return err
	}
	hand.logger.Info("processLambdaInvocation", lambdaAPIHostNamePort, nil,
		"responding to request ID %s with %d bytes: %s", awsRequestID, len(invocResult), string(invocResult))
	invokResultResp, err := inet.DoHTTP(
		context.TODO(),
		inet.HTTPRequest{Method: http.MethodPost, ContentType: "application/json", Body: bytes.NewReader(invocResult)},
		fmt.Sprintf("http://%s/2018-06-01/runtime/invocation/%s/response", lambdaAPIHostNamePort, awsRequestID))
	if err != nil {
		return err
	}
	if err := invokResultResp.Non2xxToError(); err != nil {
		return err
	}
	return nil
}

/*
decodeAndHandleHTTPRequest proxies the HTTP request deserialised and decoded from lambda invocation event to laitos web server,
and returns a lambda invocation response that encapsulates web server's response.
*/
func (hand *Handler) decodeAndHandleHTTPRequest(awsRequestID string, invocationJSON []byte, webServerPort int) (lambdaResponse []byte, err error) {
	var input InvocationInput
	if err = json.Unmarshal(invocationJSON, &input); err != nil {
		return
	}
	if input.StageVariables != nil {
		if decryptionPass := input.StageVariables[DecryptionPasswordStageVar]; decryptionPass != "" && !IsProgramDataDecrypted {
			IsProgramDataDecrypted = true
			// Use environment variable PORT to tell HTTP (not HTTPS) server to listen on port expected by lambda handler
			os.Setenv("PORT", strconv.Itoa(UpstreamWebServerPort))
			// The main function awaits program data password
			hand.logger.Info("decodeAndHandleHTTPRequest", awsRequestID, err, "decrypting program data using a password %d bytes long", len(decryptionPass))
			// Even though the channel is not buffered, HTTP server is not going to be ready immediately.
			misc.ProgramDataDecryptionPasswordInput <- decryptionPass
		}
	}
	// Prepare HTTP request for laitos web server
	var reqBody []byte
	if input.IsBase64Encoded {
		reqBody, err = base64.StdEncoding.DecodeString(input.Body)
		if err != nil {
			hand.logger.Warning("decodeAndHandleHTTPRequest", awsRequestID, err, "failed to decode base64-encoded request body")
			return
		}
	} else {
		reqBody = []byte(input.Body)
	}
	reqParams := inet.HTTPRequest{
		// Be very generous with the constraints
		TimeoutSec: 60,
		MaxBytes:   64 * 1024 * 1024,
		MaxRetry:   10,
		Method:     input.RequestContext.HTTPMethod,
		Header:     input.MultiValueHeaders,
		Body:       bytes.NewReader(reqBody),
	}
	reqURL := fmt.Sprintf("http://localhost:%d%s", webServerPort, input.RequestContext.Path)
	if len(input.MultiValueQueryStringParameters) > 0 {
		reqURL += "?" + input.MultiValueQueryStringParameters.Encode()
	}
	hand.logger.Info("decodeAndHandleHTTPRequest", awsRequestID, err, "%s %s with %d bytes of request body", reqParams.Method, reqURL, len(reqBody))
	// Send the request forth to laitos web server
	resp, err := inet.DoHTTP(context.TODO(), reqParams, strings.Replace(reqURL, "%", "%%", -1))
	if err != nil {
		hand.logger.Warning("decodeAndHandleHTTPRequest", awsRequestID, err, "failed to reach laitos web server")
		return
	}
	// Formulate the response for AWS lambda
	invocationOutput := InvocationOutput{
		StatusCode: resp.StatusCode,
		// Taking a shortcut, the response is always encoded in base64, even if it is text.
		// Remember to visit API gateway settings and set "Binary Media Types" to */*, otherwise API gateway won't attempt to decode base64.
		IsBase64Encoded: true,
		Body:            base64.StdEncoding.EncodeToString(resp.Body),
	}
	// API gateway cannot work with multi-value headers, pick up one value from each response header.
	invocationOutput.Headers = make(map[string]string)
	for headerName, headerValues := range resp.Header {
		if len(headerValues) > 0 {
			invocationOutput.Headers[headerName] = headerValues[0]
		}
	}
	lambdaResponse, err = json.Marshal(invocationOutput)
	return
}
