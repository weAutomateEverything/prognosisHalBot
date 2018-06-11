// Code generated by go-swagger; DO NOT EDIT.

package alert

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"

	models "github.com/weAutomateEverything/prognosisHalBot/models"
)

// SendImageAlertReader is a Reader for the SendImageAlert structure.
type SendImageAlertReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *SendImageAlertReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewSendImageAlertOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	default:
		result := NewSendImageAlertDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewSendImageAlertOK creates a SendImageAlertOK with default headers values
func NewSendImageAlertOK() *SendImageAlertOK {
	return &SendImageAlertOK{}
}

/*SendImageAlertOK handles this case with default header values.

Message Sent successfully
*/
type SendImageAlertOK struct {
}

func (o *SendImageAlertOK) Error() string {
	return fmt.Sprintf("[POST /api/alert/{chatid}/image][%d] sendImageAlertOK ", 200)
}

func (o *SendImageAlertOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewSendImageAlertDefault creates a SendImageAlertDefault with default headers values
func NewSendImageAlertDefault(code int) *SendImageAlertDefault {
	return &SendImageAlertDefault{
		_statusCode: code,
	}
}

/*SendImageAlertDefault handles this case with default header values.

unexpected error
*/
type SendImageAlertDefault struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the send image alert default response
func (o *SendImageAlertDefault) Code() int {
	return o._statusCode
}

func (o *SendImageAlertDefault) Error() string {
	return fmt.Sprintf("[POST /api/alert/{chatid}/image][%d] sendImageAlert default  %+v", o._statusCode, o.Payload)
}

func (o *SendImageAlertDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
