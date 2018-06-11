// Code generated by go-swagger; DO NOT EDIT.

package operations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"
)

// New creates a new operations API client.
func New(transport runtime.ClientTransport, formats strfmt.Registry) *Client {
	return &Client{transport: transport, formats: formats}
}

/*
Client for operations API
*/
type Client struct {
	transport runtime.ClientTransport
	formats   strfmt.Registry
}

/*
InvokeCallout invokes callout by sending a telegram message to the telegram group specified by the chat id

If JIRA has been configured, a JIRA ticket will be created
If CALLOUT has been defined, then the bot will invoke callout via alexa
*/
func (a *Client) InvokeCallout(params *InvokeCalloutParams) (*InvokeCalloutOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewInvokeCalloutParams()
	}

	result, err := a.transport.Submit(&runtime.ClientOperation{
		ID:                 "invokeCallout",
		Method:             "POST",
		PathPattern:        "/callout/{chatid}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &InvokeCalloutReader{formats: a.formats},
		Context:            params.Context,
		Client:             params.HTTPClient,
	})
	if err != nil {
		return nil, err
	}
	return result.(*InvokeCalloutOK), nil

}

// SetTransport changes the transport on the client
func (a *Client) SetTransport(transport runtime.ClientTransport) {
	a.transport = transport
}