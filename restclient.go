// Copyright (c) 2012 Jason McVetta.  This is Free Software, released under the 
// terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.

// Package restclient provides a simple client library for interacting with
// RESTful APIs.
package restclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"time"
)

// A Method is an HTTP verb.
type Method string

var (
	GET    = Method("GET")
	PUT    = Method("PUT")
	POST   = Method("POST")
	DELETE = Method("DELETE")
)

// A RestRequest describes an HTTP request to be executed, and the data
// structures into which results and errors will be unmarshalled.
type Request struct {
	Url      string            // Raw URL string
	Method   Method            // HTTP method to use 
	Userinfo *url.Userinfo     // Optional username/password to authenticate this request
	Params   map[string]string // URL parameters for GET requests (ignored otherwise)
	Headers  *http.Header      // HTTP Headers to use (will override defaults)
	Data     interface{}       // Data to JSON-encode and include with call
	Result   interface{}       // JSON-encoded data in respose will be unmarshalled into Result
	Error    interface{}       // If server returns error status, JSON-encoded response data will be unmarshalled into Error
}

type Response struct {
	Status    int         // HTTP status for executed request
	Timestamp time.Time   // Time the request was executed
	Result    interface{} // JSON-encoded data in respose will be unmarshalled into Result
	Error     interface{} // If server returns error status, JSON-encoded response data will be unmarshalled into Error
	RawText   string      // Gets populated with raw text of server response
	Request   *Request
}

// Client is a REST client.
type Client struct {
	HttpClient   *http.Client
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
	if req.Error == nil {
		req.Error = c.DefaultError
	}
	//
	// Create a URL object from the raw url string.  This will allow us to compose
	// query parameters programmatically and be guaranteed of a well-formed URL.
	//
	u, err := url.Parse(req.Url)
	if err != nil {
		log.Println(err)
		return nil, err
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
			return nil, err
		}
		buf := bytes.NewBuffer(b)
		hReq, err = http.NewRequest(m, u.String(), buf)
		hReq.Header.Add("Content-Type", "application/json")
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}
	//
	// If Accept header is unset, set it for JSON.
	//
	if hReq.Header.Get("Accept") == "" {
		hReq.Header.Add("Accept", "application/json")
	}
	//
	// Set HTTP Basic authentication if userinfo is supplied
	//
	if req.Userinfo != nil {
		pwd, _ := req.Userinfo.Password()
		hReq.SetBasicAuth(req.Userinfo.Username(), pwd)
	}
	//
	// Execute the HTTP request
	//
	hResp, err := c.HttpClient.Do(hReq)
	if err != nil {
		complain(err, hResp.StatusCode, "")
		return nil, err
	}
	resp := &Response{
		Status: hResp.StatusCode,
		Result: req.Result,
		Error:  req.Error,
	}
	var data []byte
	data, err = ioutil.ReadAll(hResp.Body)
	if err != nil {
		complain(err, resp.Status, string(data))
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
		log.Println(resp.Status)
		log.Println(err)
		log.Println(resp.RawText)
		log.Println(hResp)
		log.Println(hResp.Request)
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

// complain prints detailed error messages to the log.
func complain(err error, status int, rawtext string) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	lineNo := strconv.Itoa(line)
	s := "Error executing REST request:\n"
	s += "    --> Called from " + file + ":" + lineNo + "\n"
	s += "    --> Got status " + strconv.Itoa(status) + "\n"
	if rawtext != "" {
		s += "    --> Raw text of server response: " + rawtext + "\n"
	}
	s += "    --> " + err.Error()
	log.Println(s)
}

var (
	defaultClient = New()
)

// Do executes a REST request using the default client.
func Do(r *Request) (*Response, error) {
	return defaultClient.Do(r)
}
