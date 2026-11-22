package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// Отдельная сессия сохраняет приложения после завершения start.command.
func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: launcher delivery|worker|scheduler|api")
	}
	name := os.Args[1]
	switch name {
	case "delivery", "worker", "scheduler", "api":
	default:
		log.Fatal("unknown application")
	}
	self, e := os.Executable()
	if e != nil {
		log.Fatal(e)
	}
	root := filepath.Dir(filepath.Dir(self))
	runtime := filepath.Join(root, ".runtime")
	if e = os.MkdirAll(runtime, 0755); e != nil {
		log.Fatal(e)
	}
	logFile, e := os.OpenFile(filepath.Join(runtime, name+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if e != nil {
		log.Fatal(e)
	}
	defer logFile.Close()
	cmd := exec.Command(filepath.Join(root, "bin", name))
	cmd.Dir = root
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if e = cmd.Start(); e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(runtime, name+".pid"), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); e != nil {
		cmd.Process.Kill()
		log.Fatal(e)
	}
	fmt.Printf("%s started (PID %d)\n", name, cmd.Process.Pid)
	cmd.Process.Release()
}
