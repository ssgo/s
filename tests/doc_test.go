package tests

import (
	"testing"

	"github.com/ssgo/s"
)

func TestDoc(tt *testing.T) {
	t := s.T(tt)

	s.ResetAllSets()
	s.Register(0, "/echo1", Echo1, "")
	s.Register(0, "/echo2", Echo2, "")
	s.Register(0, "/echo3", Echo3, "")
	s.Register(0, "/echo4", Echo4, "")

	s.Restful(0, "GET", "/api/echo0", Echo2, "")
	s.Restful(1, "POST", "/api/echo1", Echo2, "")
	s.Restful(2, "DELETE", "/api/echo2", Echo2, "")

	s.Restful(1, "GET", "/aaa/{name}", Echo2, "")

	s.Rewrite("/r3/1", "/echo2")
	s.Rewrite("/r3\\?name=(\\w+)", "/echo/$1")

	s.Proxy(0, "/dc1/s1", "a1", "/dc/s1")
	s.Proxy(0, "/proxy/(.+?)", "a1", "/dc/$1")
	//func RegisterWebsocket(authLevel int, path string,
	//	onOpen interface{},
	//onClose interface{},
	//decoder func(data interface{}) (action string, request map[string]interface{}, err error),
	//encoder func(action string, data interface{}) interface{}) *ActionRegister {
	//return RegisterWebsocketWithPriority(authLevel, 0, path, nil, onOpen, onClose, decoder, encoder, false)
	//}

	echoAR := s.RegisterWebsocket(0, "/echoService/{token}/{roomId}", nil, OnEchoOpen, OnEchoClose, EchoDecoder, EchoEncoder, "")
	echoAR.RegisterAction(0, "", OnEchoMessage, "")

	echoAR.RegisterAction(1, "do1", func(in struct{ Name string }) (out struct{ Name string }) {
		out.Name = in.Name + "!"
		return
	}, "")

	api, _ := s.MakeDocument()
	//docBytes, _ := json.MarshalIndent(api, "", "\t")
	//docBytes, _ := json.Marshal(api)
	//u.FixUpperCase(docBytes, nil)
	//fmt.Println(string(docBytes))
	//t.Test(true, "json doc")
	t.Test(len(api) > 0, "json doc")

	s.MakeHtmlDocumentFromFile("测试文档", "doc.html", "/Volumes/Data/Case/com.isstar/ssgo/s/DocTpl.html")
	//fmt.Println(u.ReadFile("doc.html", 10240))
}
