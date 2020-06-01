package lambda

import (
	"net/url"
)

// RequestContext is a component of HTTP request coming from AWS API gateway.
type RequestContext struct {
	Stage      string `json:"stage"`
	Path       string `json:"path"`
	HTTPMethod string `json:"httpMethod"`
}

// InvocationInput describes an HTTP request coming from AWS API gateway to be processed by lambda function.
type InvocationInput struct {
	StageVariables                  map[string]string   `json:"stageVariables"`
	RequestContext                  RequestContext      `json:"requestContext"`
	MultiValueQueryStringParameters url.Values          `json:"multiValueQueryStringParameters"`
	MultiValueHeaders               map[string][]string `json:"multiValueHeaders"`
	IsBase64Encoded                 bool                `json:"isBase64Encoded"`
	Body                            string              `json:"body"`
}

// InvocationOutput describes a lambda function response translated from laitos web server response.
type InvocationOutput struct {
	StatusCode int `json:"statusCode"`
	/*
		Headers are HTTP response headers for API gateway.
		Contrary to convention, API gateway consider multi-value header as malformed.
	*/
	Headers         map[string]string `json:"headers"`
	IsBase64Encoded bool              `json:"isBase64Encoded"`
	Body            string            `json:"body"`
}
