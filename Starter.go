package s

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func init() {
	os.Chdir(os.Args[0][0:strings.LastIndexByte(os.Args[0], '/')])

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
		}
	}
}

func loadPid() (string, int) {
	app := os.Args[0][strings.LastIndexByte(os.Args[0], '/')+1:]
	pidFile, err := os.Open("/tmp/" + app + ".pid")
	if err == nil {
		b := make([]byte, 10)
		n, err := pidFile.Read(b)
		if err == nil {
			pid, err := strconv.Atoi(string(b[0:n]))
			if err == nil {
				return app, pid
			}
		}
		pidFile.Close()
	}
	return app, 0
}

func savePid(app string, pid int) {
	pidFile, err := os.OpenFile("/tmp/"+app+".pid", os.O_CREATE|os.O_WRONLY, 0600)
	if err == nil {
		pidFile.Write([]byte(strconv.Itoa(pid)))
		pidFile.Close()
	}
}

func startProcess() {
	app, pid := loadPid()
	if pid > 0 {
		fmt.Printf("%s	%d	is already running, please stop first\n", app, pid)
		return
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
	savePid(app, cmd.Process.Pid)
	fmt.Printf("%s	%d	is running...\n", app, cmd.Process.Pid)
}

func stopProcess() {
	app, pid := loadPid()
	if pid <= 0 {
		fmt.Printf("%s	not run\n", app)
		return
	}
	cmd := exec.Command("kill", strconv.Itoa(pid))
	out, err := cmd.Output()
	fmt.Println(string(out))
	if err != nil {
		fmt.Println(err)
	}
	os.Remove("/tmp/" + app + ".pid")
	fmt.Printf("%s	%d	stopped\n", app, cmd.Process.Pid)
}

func statusProcess() {
	app, pid := loadPid()
	if pid <= 0 {
		fmt.Printf("%s	not run\n", app)
		return
	}
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid))
	out, err := cmd.Output()
	fmt.Println(string(out))
	if err != nil {
		fmt.Println(err)
	}
}
