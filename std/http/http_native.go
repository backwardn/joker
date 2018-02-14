package http

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/candid82/joker/core"
)

var client = &http.Client{}

func extractMethod(request Map) string {
	if ok, m := request.Get(MakeKeyword("method")); ok {
		switch m := m.(type) {
		case String:
			return m.S
		case Keyword:
			return m.ToString(false)[1:]
		case Symbol:
			return m.ToString(false)
		default:
			panic(RT.NewError(fmt.Sprintf("method must be a string, keyword or symbol, got %s", m.GetType().ToString(false))))
		}
	}
	return "get"
}

func getOrPanic(m Map, k Object, errMsg string) Object {
	if ok, v := m.Get(k); ok {
		return v
	}
	panic(RT.NewError(errMsg))
}

func mapToReq(request Map) *http.Request {
	method := strings.ToUpper(extractMethod(request))
	url := AssertString(getOrPanic(request, MakeKeyword("url"), ":url key must be present in request map"), "url must be a string").S
	var reqBody io.Reader
	if ok, b := request.Get(MakeKeyword("body")); ok {
		reqBody = strings.NewReader(AssertString(b, "body must be a string").S)
	}
	req, err := http.NewRequest(method, url, reqBody)
	PanicOnErr(err)
	if ok, headers := request.Get(MakeKeyword("headers")); ok {
		h := AssertMap(headers, "headers must be a map")
		for iter := h.Iter(); iter.HasNext(); {
			p := iter.Next()
			req.Header.Add(AssertString(p.Key, "header name must be a string").S, AssertString(p.Value, "header value must be a string").S)
		}
	}
	if ok, host := request.Get(MakeKeyword("host")); ok {
		req.Host = AssertString(host, "host must be a string").S
	}
	return req
}

func reqToMap(host String, port String, req *http.Request) Map {
	defer req.Body.Close()
	res := EmptyArrayMap()
	body, err := ioutil.ReadAll(req.Body)
	PanicOnErr(err)
	res.Add(MakeKeyword("request-method"), MakeKeyword(strings.ToLower(req.Method)))
	res.Add(MakeKeyword("body"), MakeString(string(body)))
	res.Add(MakeKeyword("uri"), MakeString(req.URL.Path))
	res.Add(MakeKeyword("query-string"), MakeString(req.URL.RawQuery))
	res.Add(MakeKeyword("server-name"), host)
	res.Add(MakeKeyword("server-port"), port)
	res.Add(MakeKeyword("remote-addr"), MakeString(req.RemoteAddr[:strings.LastIndexByte(req.RemoteAddr, byte(':'))]))
	res.Add(MakeKeyword("protocol"), MakeString(req.Proto))
	res.Add(MakeKeyword("scheme"), MakeKeyword("http"))
	headers := EmptyArrayMap()
	for k, v := range req.Header {
		headers.Add(MakeString(strings.ToLower(k)), MakeString(strings.Join(v, ",")))
	}
	res.Add(MakeKeyword("headers"), headers)
	return res
}

func respToMap(resp *http.Response) Map {
	defer resp.Body.Close()
	res := EmptyArrayMap()
	body, err := ioutil.ReadAll(resp.Body)
	PanicOnErr(err)
	res.Add(MakeKeyword("body"), MakeString(string(body)))
	res.Add(MakeKeyword("status"), MakeInt(resp.StatusCode))
	respHeaders := EmptyArrayMap()
	for k, v := range resp.Header {
		respHeaders.Add(MakeString(k), MakeString(strings.Join(v, ",")))
	}
	res.Add(MakeKeyword("headers"), respHeaders)
	res.Add(MakeKeyword("content-length"), MakeInt(int(resp.ContentLength)))
	return res
}

func mapToResp(response Map, w http.ResponseWriter) {
	if ok, b := response.Get(MakeKeyword("body")); ok {
		io.WriteString(w, AssertString(b, "HTTP response body must be a string").S)
	}
}

func sendRequest(request Map) Map {
	resp, err := client.Do(mapToReq(request))
	PanicOnErr(err)
	return respToMap(resp)
}

func startServer(addr string, handler Callable) Object {
	i := strings.LastIndexByte(addr, byte(':'))
	host, port := MakeString(addr), MakeString("")
	if i != -1 {
		host = MakeString(addr[:i])
		port = MakeString(addr[i+1:])
	}
	err := http.ListenAndServe(addr, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		response := handler.Call([]Object{reqToMap(host, port, req)})
		mapToResp(AssertMap(response, "HTTP response must be a map"), w)
	}))
	PanicOnErr(err)
	return NIL
}