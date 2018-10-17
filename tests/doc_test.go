package tests

import (
	"testing"

	".."
)

func TestDoc(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(0, "/echo1", Echo1)
	s.Register(0, "/echo2", Echo2)
	s.Register(0, "/echo3", Echo3)
	s.Register(0, "/echo4", Echo4)

	s.Restful(0, "GET", "/api/echo0", Echo2)
	s.Restful(1, "POST", "/api/echo1", Echo2)
	s.Restful(2, "DELETE", "/api/echo2", Echo2)

	s.Restful(1, "GET", "/aaa/{name}", Echo2)

	s.Rewrite("/r3/1", "/echo2")
	s.Rewrite("/r3\\?name=(\\w+)", "/echo/$1")

	s.Proxy("/dc1/s1", "a1", "/dc/s1")
	s.Proxy("/proxy/(.+?)", "a1", "/dc/$1")

	echoAR := s.RegisterWebsocket(0, "/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder, EchoEncoder)
	echoAR.RegisterAction(0, "", OnEchoMessage)

	echoAR.RegisterAction(1, "do1", func(in struct{ Name string }) (out struct{ Name string }) {
		out.Name = in.Name + "!"
		return
	})

	doc := s.MakeDocument()
	//docBytes, _ := json.MarshalIndent(doc, "", "\t")
	//fmt.Println(string(docBytes))
	//t.Test(true, "json doc")
	t.Test(len(doc) > 0, "json doc")

	s.MakeHtmlDocumentFile("测试文档", "doc.html")
}
