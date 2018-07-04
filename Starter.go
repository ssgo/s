package s

import (
	"fmt"
	"github.com/ssgo/httpclient"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type serviceInfoType struct {
	pidFile     string
	pid         int
	httpVersion int
	baseUrl     string
}

func (si *serviceInfoType) exists() bool {
	fi, err := os.Stat(si.pidFile)
	return err == nil && fi != nil
}
func (si *serviceInfoType) remove() {
	if si.exists() {
		os.Remove(si.pidFile)
	}
}
func (si *serviceInfoType) save() {
	pidFile, err := os.OpenFile(si.pidFile, os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		pidFile.Write([]byte(fmt.Sprintf("%d,%d,%s", si.pid, si.httpVersion, si.baseUrl)))
		pidFile.Close()
	}
}
func (si *serviceInfoType) load() {
	pidFile, err := os.Open(si.pidFile)
	if err == nil {
		b := make([]byte, 1024)
		n, err := pidFile.Read(b)
		if err == nil {
			a := strings.SplitN(string(b[0:n]), ",", 3)
			pid, err := strconv.Atoi(a[0])
			if err == nil {
				si.pid = pid
			}
			if len(a) > 1 {
				httpVersion, err := strconv.Atoi(a[1])
				if err == nil {
					si.httpVersion = httpVersion
				}
				if si.httpVersion != 1 {
					si.httpVersion = 2
				}
			}
			if len(a) > 2 {
				si.baseUrl = a[2]
			}
		}
		pidFile.Close()
	}
}

var serviceInfo serviceInfoType

func init() {
	// 不切换方便开发，生产环境注意路径，尽量使用绝对路径
	//os.Chdir(os.Args[0][0:strings.LastIndexByte(os.Args[0], os.PathSeparator)])
	serviceInfo = serviceInfoType{pidFile: "/tmp/" + strings.Replace(os.Args[0], "/", "_", 100) + ".pid"}
	serviceInfo.load()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "start", "1":
			startProcess()
			os.Exit(0)
		case "stop", "0":
			stopProcess()
			os.Exit(0)
		case "restart", "2":
			stopProcess()
			startProcess()
			os.Exit(0)
		case "status", "s":
			statusProcess()
			os.Exit(0)
		case "check", "c":
			checkProcess()
			os.Exit(0)
		}
	}
}

//func loadPid() (string, int) {
//	//app := os.Args[0][strings.LastIndexByte(os.Args[0], '/')+1:]
//	app := os.Args[0]
//	pidFile, err := os.Open("/tmp/" + strings.Replace(os.Args[0], "/", "_", 100) + ".pid")
//	if err == nil {
//		b := make([]byte, 10)
//		n, err := pidFile.Read(b)
//		if err == nil {
//			pid, err := strconv.Atoi(string(b[0:n]))
//			if err == nil {
//				return app, pid
//			}
//		}
//		pidFile.Close()
//	}
//	return app, 0
//}

//func savePid(app string, pid int) {
//}

func startProcess() {
	if serviceInfo.pid > 0 {
		fmt.Printf("%s	%d	is already running, stopping ...\n", os.Args[0], serviceInfo.pid)
		stopProcess()
	}

	var cmd *exec.Cmd
	if len(os.Args) > 2 {
		cmd = exec.Command(os.Args[0], os.Args[2:]...)
	} else {
		cmd = exec.Command(os.Args[0])
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()

	if !serviceInfo.exists() {
		// 如果进程内没有产生pid文件，保存
		serviceInfo.pid = cmd.Process.Pid
		serviceInfo.save()
	}
	fmt.Printf("%s	%d	is running...\n", os.Args[0], cmd.Process.Pid)
}

func stopProcess() {
	if serviceInfo.pid <= 0 {
		fmt.Printf("%s	not run\n", os.Args[0])
		return
	}
	cmd := exec.Command("kill", strconv.Itoa(serviceInfo.pid))
	out, err := cmd.Output()
	fmt.Println(string(out))
	if err != nil {
		fmt.Println(err)
	}
	serviceInfo.remove()
	fmt.Printf("%s	%d	stopped\n", os.Args[0], cmd.Process.Pid)
}

func statusProcess() {
	if serviceInfo.pid <= 0 {
		fmt.Printf("%s	not run\n", os.Args[0])
		return
	}
	cmd := exec.Command("ps", "-o", "pid,user,stat,start,time,args,%cpu,%mem", "-p", strconv.Itoa(serviceInfo.pid))
	out, err := cmd.Output()
	fmt.Println(string(out))
	if err != nil {
		fmt.Println(err)
	}
}

func checkProcess() {
	if serviceInfo.pid <= 0 {
		fmt.Printf("%s	not run\n", os.Args[0])
		os.Exit(1)
		return
	}
	pid := strconv.Itoa(serviceInfo.pid)
	cmd := exec.Command("ps", "-p", pid)
	outBytes, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	out := string(outBytes)
	if strings.Index(out, "\n"+pid+" ") == -1 && strings.Index(out, " "+pid+" ") == -1 {
		fmt.Printf("pid: %s not found\n", pid)
		fmt.Println(string(out))
		os.Exit(1)
		return
	}

	var client *httpclient.ClientPool
	if serviceInfo.httpVersion == 1 {
		client = httpclient.GetClient(3000)
	} else {
		client = httpclient.GetClientH2C(3000)
	}

	checkUrl := serviceInfo.baseUrl + "/__CHECK__"
	r := client.Head(checkUrl, nil, "Pid", pid)
	if r.Error != nil {
		fmt.Printf("request %s error %s\n", checkUrl, r.Error.Error())
		os.Exit(1)
		return
	}

	if r.Response.StatusCode != 299 {
		fmt.Printf("check %s error %d\n", checkUrl, r.Response.StatusCode)
		os.Exit(1)
		return
	}

	fmt.Print("check ok\n")
	os.Exit(0)
}
