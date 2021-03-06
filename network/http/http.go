/**********************************************************************************
* Copyright (c) 2009-2017 Misakai Ltd.
* This program is free software: you can redistribute it and/or modify it under the
* terms of the GNU Affero General Public License as published by the  Free Software
* Foundation, either version 3 of the License, or(at your option) any later version.
*
* This program is distributed  in the hope that it  will be useful, but WITHOUT ANY
* WARRANTY;  without even  the implied warranty of MERCHANTABILITY or FITNESS FOR A
* PARTICULAR PURPOSE.  See the GNU Affero General Public License  for  more details.
*
* You should have  received a copy  of the  GNU Affero General Public License along
* with this program. If not, see<http://www.gnu.org/licenses/>.
************************************************************************************/

package http

import (
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/emitter-io/emitter/utils"
	"github.com/valyala/fasthttp"
)

// HeaderValue represents a header with a value attached.
type HeaderValue struct {
	Header string
	Value  string
}

// NewHeader builds an HTTP header with a value.
func NewHeader(header, value string) HeaderValue {
	return HeaderValue{Header: header, Value: value}
}

// Client represents an HTTP client which can be used for issuing requests concurrently.
type Client interface {
	Get(url string, output interface{}, headers ...HeaderValue) ([]byte, error)
	Post(url string, body []byte, output interface{}, headers ...HeaderValue) ([]byte, error)
}

// Client implementation.
type client struct {
	host string               // The host name of the client.
	http *fasthttp.HostClient // The underlying client.
	head []HeaderValue        // The default headers to add on each request.
}

// NewClient creates a new HTTP Client for the provided host. This will use round-robin
// to load-balance the requests to the addresses resolved by the host.
func NewClient(host string, timeout time.Duration, defaultHeaders ...HeaderValue) (Client, error) {

	// Parse the URL
	u, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	// Get the addresses by performing a DNS lookup, this should not fail
	addr, err := net.LookupHost(u.Hostname())
	if err != nil {
		return nil, err
	}

	// Add port to each address
	for i, a := range addr {
		addr[i] = a + ":" + u.Port()
	}

	// Construct a new client
	c := new(client)
	c.host = host
	c.head = defaultHeaders
	c.http = &fasthttp.HostClient{
		Addr:         strings.Join(addr, ","),
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	return c, nil
}

// Get issues an HTTP Get on a specified URL and decodes the payload as JSON.
func (c *client) Get(url string, output interface{}, headers ...HeaderValue) ([]byte, error) {
	return c.do(url, "GET", nil, output, headers)
}

// Post is a utility function which marshals and issues an HTTP post on a specified URL.
func (c *client) Post(url string, body []byte, output interface{}, headers ...HeaderValue) ([]byte, error) {
	return c.do(url, "POST", body, output, headers)
}

// This performs a request
func (c *client) do(url, method string, body []byte, output interface{}, headers []HeaderValue) (responseBody []byte, err error) {

	// Prepare the request
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI(url)

	// Set body if provided
	if body != nil {
		req.SetBody(body)
	}

	// Set the default headers
	for _, h := range c.head {
		req.Header.Set(h.Header, h.Value)
	}

	// Set the headers
	req.Header.SetMethod(method)
	req.Header.Set("Accept", "application/json, application/binary")
	for _, h := range headers {
		req.Header.Set(h.Header, h.Value)
	}

	// Acquire a response
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	// Issue the request
	err = c.http.Do(req, res)
	if err == nil {
		// Set the response body
		responseBody = res.Body()

		// Decode if necessary
		if output != nil {
			// Get the content type
			mime := string(res.Header.ContentType())
			switch mime {
			case "application/binary":
				err = utils.Decode(res.Body(), output)

			default:
				// Always default to JSON here
				err = json.Unmarshal(res.Body(), output)
			}
		}
	}
	return
}
