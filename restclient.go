// Copyright (c) 2012 Jason McVetta.  This is Free Software, released under the 
// terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.

// Package restclient provides a simple client library for interacting with
// RESTful APIs.
package restclient

import (
	"bytes"
	"log"
	"runtime"
	"strconv"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Method string

var (
	GET    = Method("GET")
	PUT    = Method("PUT")
	POST   = Method("POST")
	DELETE = Method("DELETE")
)

type Request struct {
	Url     string            // Raw URL string
	Method  Method            // HTTP method to use 
	Params  map[string]string // URL parameters for GET requests (ignored otherwise)
	Headers *http.Header      // HTTP Headers to use (will override defaults)
	Data    interface{}       // Data to JSON-encode and include with call
	Result  interface{}       // JSON-encoded data in respose will be unmarshalled into Result
	Error   interface{}       // If server returns error status, JSON-encoded response data will be unmarshalled into Error
}

type Response struct {
	Request *Request // Request used to generate this response
	Status	int  // HTTP status for executed request
	Result  interface{}       // JSON-encoded data in respose will be unmarshalled into Result
	Error   interface{}       // If server returns error status, JSON-encoded response data will be unmarshalled into Error
	RawText string            // Gets populated with raw text of server response
}

// Client is a REST client.
type Client struct {
	HttpClient *http.Client
	DefaultError interface{}
}

// New returns a new Client instance.
func New() *Client {
	return &Client{
		HttpClient: new(http.Client),
	}
}

// Do executes a REST request.
func (c *Client) Do(req *Request) (*Response, error) {
	resp := new(Response)
	resp.Request = req
	resp.Result = req.Result
	resp.Error = req.Error
	//
	// Create a URL object from the raw url string.  This will allow us to compose
	// query parameters programmatically and be guaranteed of a well-formed URL.
	//
	u, err := url.Parse(req.Url)
	if err != nil {
		log.Println(err)
		return resp, err
	}
	//
	// If we are making a GET request and the user populated the Params field, then
	// add the params to the URL's querystring.
	//
	if req.Method == GET && req.Params != nil {
		vals := u.Query()
		for k, v := range req.Params {
			vals.Set(k, v)
		}
		u.RawQuery = vals.Encode()
	}
	//
	// Create a Request object; if populated, Data field is JSON encoded as request
	// body
	//
	m := string(req.Method)
	var hReq *http.Request
	if req.Data == nil {
		hReq, err = http.NewRequest(m, u.String(), nil)
	} else {
		var b []byte
		b, err = json.Marshal(req.Data)
		if err != nil {
			log.Println(err)
			return resp, err
		}
		buf := bytes.NewBuffer(b)
		hReq, err = http.NewRequest(m, u.String(), buf)
		hReq.Header.Add("Content-Type", "application/json")
	}
	if err != nil {
		log.Println(err)
		return resp, err
	}
	//
	// If Accept header is unset, set it for JSON.
	//
	if hReq.Header.Get("Accept") == "" {
		hReq.Header.Add("Accept", "application/json")
	}
	//
	// Execute the HTTP request
	//
	hResp, err := c.HttpClient.Do(hReq)
	if err != nil {
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "???"
			line = 0
		}
		lineNo := strconv.Itoa(line)
		s := "Error executing REST request:\n"
		s += "\t Called from " + file + ":" + lineNo + "\n"
		s += "\t "
		log.Println(s, err)
		return resp, err
	}
	resp.Status = hResp.StatusCode
	var data []byte
	data, err = ioutil.ReadAll(hResp.Body)
	if err != nil {
		log.Println(err)
		return resp, err
	}
	resp.RawText = string(data)
	// If server returned no data, don't bother trying to unmarshall it (which will fail anyways).
	if resp.RawText == "" {
		return resp, err
	}
	if resp.Status >= 200 && resp.Status < 300 {
		err = c.unmarshal(data, &resp.Result)
	} else {
		err = c.unmarshal(data, &resp.Error)
	}
	if err != nil {
		log.Println(resp)
		log.Println(err)
	}
	return resp, err
}

// unmarshal parses the JSON-encoded data and stores the result in the value
// pointed to by v.  If the data cannot be unmarshalled without error, v will be 
// reassigned the value interface{}, and data unmarshalled into that.
func (c *Client) unmarshal(data []byte, v interface{}) error {
	err := json.Unmarshal(data, v)
	if err == nil {
		return nil
	}
	v = new(interface{})
	return json.Unmarshal(data, v)
}
