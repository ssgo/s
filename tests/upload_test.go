package tests

import (
	"github.com/ssgo/httpclient"
	"github.com/ssgo/log"
	"github.com/ssgo/s"
	"github.com/ssgo/u"
	"os"
	"testing"
	"time"
)

func TestUpload(tt *testing.T) {
	t := s.T(tt)

	file1Tag := ""
	file1Buf := make([]byte, 0)
	_ = os.Setenv("service_listen", ":,http")
	s.ResetAllSets()
	s.Restful(0, "POST", "/upload", func(in struct {
		Tag1, Tag2   string
		File1, File2 *s.UploadFile
	}, logger *log.Logger) bool {
		file1Tag = in.Tag1
		var err error
		if file1Buf, err = in.File1.Content(); err != nil {
			logger.Error(err.Error())
		}
		if err = in.File2.Save("downloads/22.txt"); err != nil {
			logger.Error(err.Error())
		}
		return err == nil
	}, "")
	s.Restful(0, "GET", "/download1", func(response *s.Response) {
		response.Header().Set("Tag", file1Tag)
		response.DownloadFile("", "file", file1Buf)
	}, "")
	s.Static("/downloads/", "downloads/")
	as := s.AsyncStart()

	u.WriteFile("1.txt", "111")
	u.WriteFile("2.txt", "012345678901234567890123456789012345678901234")
	defer os.Remove("1.txt")
	defer os.Remove("2.txt")
	c := httpclient.GetClient(time.Second * 10)
	c.DownloadPartSize = 10
	r, errors := c.MPost("http://"+as.Addr+"/upload", map[string]string{"Tag1": "tag1", "Tag2": "tag2"}, map[string]any{"file1": "1.txt", "file2": "2.txt"})
	t.Test(r.Error == nil && r.String() == "true", "Upload", r.String(), errors)

	r = c.Get("http://" + as.Addr + "/download1")
	defer os.RemoveAll("downloads")
	t.Test(r.Error == nil && r.Response.Header.Get("Tag") == "tag1" && r.String() == "111", "Download Uploaded 1", r.Response.Header.Get("Tag"), r.String(), r.Error)

	r, err := c.Download("222.txt", "http://"+as.Addr+"/downloads/22.txt", nil)
	defer os.RemoveAll("222.txt")
	buf2, err := u.ReadFile("222.txt")
	t.Test(r.Error == nil && err == nil && buf2 == "012345678901234567890123456789012345678901234", "Download Uploaded 2", buf2, err, r.Error)
}
