package s

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ssgo/u"

	"github.com/ssgo/httpclient"
)

type serviceInfoType struct {
	pidFile  string
	pid      int
	protocol string
	baseUrl  string
}

type StartCmd struct {
	Name    string
	Comment string
	Func    func()
}

var startCmds = []StartCmd{
	{"start", "Start server", startProcess},
	{"stop", "Stop server", stopProcess},
	{"restart", "Restart server", restartProcess},
	{"status", "Show status", statusProcess},
	{"check", "Check server over http request", checkProcess},
	{"doc", `Make Document
  ./server doc - print api doc with json format
  ./server doc output.json - save api doc with json format
  ./server doc output.html - save api doc with html format (default template is DocTpl.html)
  ./server doc output.html tpl.html - save api doc with html format (template is special file tpl.html)`, cmdMakeDocument},
}

func resetStarterMemory() {
	startCmds = []StartCmd{}
}

// 			makeDockment(os.Args[2], os.Args[3])
//		} else if len(os.Args) >= 3 {
//			makeDockment(os.Args[2], "")
//		} else {
//			makeDockment("", "")

func (si *serviceInfoType) exists() bool {
	fi, err := os.Stat(si.pidFile)
	return err == nil && fi != nil
}
func (si *serviceInfoType) remove() {
	if si.exists() {
		_ = os.Remove(si.pidFile)
	}
}
func (si *serviceInfoType) save() {
	pidFile, err := os.OpenFile(si.pidFile, os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		_, _ = pidFile.Write([]byte(fmt.Sprintf("%d,%s,%s", si.pid, si.protocol, si.baseUrl)))
		_ = pidFile.Close()
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
				si.protocol = a[1]
				//httpVersion, err := strconv.Atoi(a[1])
				//if err == nil {
				//	si.httpVersion = httpVersion
				//}
				//if si.httpVersion != 1 {
				//	si.httpVersion = 2
				//}
			}
			if len(a) > 2 {
				si.baseUrl = a[2]
			}
		}
		_ = pidFile.Close()
	}
}

var serviceInfo serviceInfoType

//var inDocumentMode = false

func tryStartPath(testFile string) string {
	if u.FileExists(testFile) {
		// use current path
		if path.IsAbs(testFile) {
			return path.Dir(testFile)
		} else {
			curPath, _ := os.Getwd()
			return curPath
		}
	}

	tryPath := path.Dir(shellFile)
	if u.FileExists(filepath.Join(tryPath, testFile)) {
		// use arg0 path
		return tryPath
	}

	// no set
	return ""
}

var shellFile = ""

func initStarter() {
	shellFile, _ = filepath.Abs(os.Args[0])

	if workPath == "" {
		if workPath == "" && len(os.Args) > 1 && strings.ContainsRune(os.Args[1], '.') {
			workPath = tryStartPath(os.Args[1])
		}
		if workPath == "" && len(os.Args) > 2 && strings.ContainsRune(os.Args[2], '.') {
			workPath = tryStartPath(os.Args[2])
		}
		if workPath == "" {
			workPath = tryStartPath("env.yml")
		}
		if workPath == "" {
			workPath = tryStartPath("env.json")
		}

		if workPath == "" && !strings.Contains(shellFile, "/go-build") {
			workPath = path.Dir(shellFile)
		}
	}
	if workPath != "" {
		_ = os.Chdir(workPath)
	}
	serviceInfo = serviceInfoType{pidFile: filepath.Join(workPath, ".pid")}
	serviceInfo.load()

	//if len(os.Args) > 1 {
	//	log.DefaultLogger.SetLevel(log.CLOSE)
	//}
}

func AddCmd(name, comment string, function func()) {
	startCmds = append(startCmds, StartCmd{name, comment, function})
}

func CheckCmd() {
	if len(os.Args) > 1 {
		//log.DefaultLogger.SetLevel(log.INFO)

		cmd := os.Args[1]
		if cmd == "help" || cmd == "--help" {
			fmt.Printf("%s (%s)\n\n", u.Cyan(path.Base(shellFile)), version)
			for _, cmdInfo := range startCmds {
				fmt.Printf("%s\t%s\n", u.Cyan(cmdInfo.Name), cmdInfo.Comment)
			}
			fmt.Println()
			os.Exit(0)
		} else {
			// 匹配到命令行命令
			for _, cmdInfo := range startCmds {
				if cmd == cmdInfo.Name {
					cmdInfo.Func()
					os.Exit(0)
				}
			}
		}

		//switch os.Args[1] {
		//case "start", "1":
		//	startProcess()
		//	os.Exit(0)
		//case "stop", "0":
		//	stopProcess()
		//	os.Exit(0)
		//case "restart", "2":
		//	stopProcess()
		//	startProcess()
		//	os.Exit(0)
		//case "status", "s":
		//	statusProcess()
		//	os.Exit(0)
		//case "doc":
		//	inDocumentMode = true
		//case "check", "c":
		//	checkProcess()
		//	os.Exit(0)
		//}
	}

	////os.Chdir(shellFile[0:strings.LastIndexByte(shellFile, os.PathSeparator)])
	//pos := strings.LastIndexByte(shellFile, os.PathSeparator)
	//if pos == -1 {
	//	pos = strings.LastIndexByte(shellFile, '/')
	//}
	//
	//if pos != -1 {
	//	os.Chdir(shellFile[0:pos])
	//}
}

//func loadPid() (string, int) {
//	//app := shellFile[strings.LastIndexByte(shellFile, '/')+1:]
//	app := shellFile
//	pidFile, err := os.Open("/tmp/" + strings.Replace(shellFile, "/", "_", 100) + ".pid")
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

func cmdMakeDocument() {
	//inDocumentMode = true
	if len(os.Args) >= 4 {
		makeDockment(os.Args[2], os.Args[3])
	} else if len(os.Args) >= 3 {
		makeDockment(os.Args[2], "")
	} else {
		makeDockment("", "")
	}
}

func makeDockment(toFile, fromFile string) {
	if toFile == "" {
		fmt.Println(MakeJsonDocument())
	} else if strings.HasSuffix(toFile, ".html") {
		if fromFile == "" {
			MakeHtmlDocumentFile("Api", toFile)
		} else {
			MakeHtmlDocumentFromFile("Api", toFile, fromFile)
		}
	} else {
		MakeJsonDocumentFile(toFile)
	}
}

func startProcess() {
	if serviceInfo.pid > 0 {
		fmt.Printf("%s	%d	is already running, stopping ...\n", shellFile, serviceInfo.pid)
		stopProcess()
	}

	var cmd *exec.Cmd
	if len(os.Args) > 2 {
		cmd = exec.Command(shellFile, os.Args[2:]...)
	} else {
		cmd = exec.Command(shellFile)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err == nil {
		if cmd.Process != nil && !serviceInfo.exists() {
			// 如果进程内没有产生pid文件，保存
			serviceInfo.pid = cmd.Process.Pid
			serviceInfo.save()
		}
		fmt.Printf("%s	%d	is running...\n", shellFile, serviceInfo.pid)
	} else {
		fmt.Println("failed to start process", err.Error())
	}
}

func stopProcess() {
	if serviceInfo.pid <= 0 {
		fmt.Printf("%s	not run\n", shellFile)
		return
	}
	cmd := exec.Command("kill", strconv.Itoa(serviceInfo.pid))
	out, err := cmd.Output()
	fmt.Println(string(out))
	if err != nil {
		fmt.Println(err)
	}
	serviceInfo.remove()
	fmt.Printf("%s	%d	stopped\n", shellFile, cmd.Process.Pid)
}

func restartProcess() {
	stopProcess()
	time.Sleep(time.Second)
	startProcess()
}

func statusProcess() {
	fmt.Printf("%s (%s)\n\n", u.Cyan(path.Base(shellFile)), version)
	if serviceInfo.pid <= 0 {
		fmt.Printf("%s	not run\n", shellFile)
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
		fmt.Printf("%s	not run\n", shellFile)
		os.Exit(1)
		return
	}
	pid := strconv.Itoa(serviceInfo.pid)
	cmd := exec.Command("ps", "ax -o pid|grep -E '^\\s*"+pid+"$'")
	outBytes, err := cmd.Output()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	if len(outBytes) == 0 {
		fmt.Printf("pid: %s not found\n", pid)
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
	if serviceInfo.protocol != "h2c" {
		client = httpclient.GetClient(3000)
	} else {
		client = httpclient.GetClientH2C(3000)
	}

	checkUrl := serviceInfo.baseUrl + "/__CHECK__"
	r := client.Head(checkUrl, "Pid", pid)
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
