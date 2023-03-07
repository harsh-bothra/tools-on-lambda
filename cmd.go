package main

import (
	b64 "encoding/base64"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type CommandResponse struct {
	Status int
	Output string
}

func New(cmdString string) (CommandResponse, error) {
	log.Infof("executing command : %s and setting $HOME", cmdString)
	os.Setenv("HOME", "/tmp")
	if err := os.Mkdir("/tmp/.config", os.ModePerm); err != nil {
		log.Errorf("error creating directory : %s", err)
	}
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("error executing command : %s : %s", stdoutStderr, err)
		return CommandResponse{Status: 1, Output: ""}, err
	}
	log.Infof("command's response : %s", stdoutStderr)
	return CommandResponse{Status: 0, Output: b64.StdEncoding.EncodeToString(stdoutStderr)}, nil
}

func Worker(job Job, output chan Job) {
	cmdOutput, err := New(job.CMDString)
	if err != nil {
		return
	}
	job.Output = cmdOutput.Output
	job.Status = cmdOutput.Status
	output <- job
}
