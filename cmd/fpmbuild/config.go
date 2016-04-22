// vim: ts=4:sw=4:sts=4
package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

var defaultFile FPMBuildFile = FPMBuildFile{
	Build: FPMBuildInfo{
		Prepare:   ``,
		Build:     `if [ -e Makefile ]; then make DESTDIR="$PWD/fpmroot"; fi`,
		FPMGen:    `if [ -e Makefile ]; then make DESTDIR="$PWD/fpmroot" .fpm || true; fi`,
		Install:   `if [ -e Makefile ]; then rm -rf fpmroot; make DESTDIR="$PWD/fpmroot" install; fi`,
		Shell:     `sh`,
		Options:   []string{"-c", "-xe"},
		Arguments: []string{},
	},
	Environment: FPMBuildEnvironment{
		Docker: nil,
	},
}

type FPMBuildFile struct {
	Build       FPMBuildInfo        `yaml:"build"`
	Clean       string              `yaml:"clean"`
	FPM         []string            `yaml:"fpm"`
	FPMHooks    map[string]string   `yaml:"fpm-hooks"`
	Environment FPMBuildEnvironment `yaml:"env"`
}

type FPMBuildInfo struct {
	Prepare   string   `yaml:"prepare"`
	FPMGen    string   `yaml:"fpmgen"`
	Build     string   `yaml:"build"`
	Install   string   `yaml:"install"`
	Shell     string   `yaml:"shell"`
	Options   []string `yaml:"options"`
	Arguments []string `yaml:"arguments"`
}

type FPMBuildEnvironment struct {
	Docker *DockerEnvironment `yaml:"docker"`
}

type DockerEnvironment struct {
	Image      string `yaml:"image"`
	Dockerfile string `yaml:"Dockerfile"`
	SrcPath    string `yaml:"srcpath"`
}

func (i *FPMBuildInfo) Command() []string {
	res := []string{i.Shell}
	res = append(res, i.Options...)
	res = append(res,
		"\n"+
			i.Prepare+"\n"+
			i.Build+"\n"+
			i.FPMGen+"\n"+
			i.Install+"\n")
	res = append(res, i.Arguments...)
	return res
}

type Environment interface {
	Execute(command []string) error
}

var dockerSudo bool = false

func (env *DockerEnvironment) Execute(command []string) error {
	image := env.Image
	srcPath := env.SrcPath
	dockerfile := []byte(env.Dockerfile)
	if len(dockerfile) > 0 {
		image = fmt.Sprintf("fpmbuild:%x", sha1.Sum(dockerfile))
		buildargs := []string{"docker", "build", "-t", image, "-"}
		if dockerSudo {
			buildargs = append([]string{"sudo"}, buildargs...)
		}
		log.Println(strings.Join(buildargs, " "))
		cmd := exec.Command(buildargs[0], buildargs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = bytes.NewReader(dockerfile)
		err := cmd.Run()
		if err != nil {
			return err
		}
	}
	if image == "" {
		image = "debian:stable"
	}
	if srcPath == "" {
		srcPath = "/src"
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uid := os.Getuid()
	gid := os.Getgid()
	args := []string{"docker", "run", "--rm",
		"-v", cwd + ":" + srcPath,
		"-u", fmt.Sprintf("%d:%d", uid, gid),
		"-w", srcPath,
		image}
	if dockerSudo {
		args = append([]string{"sudo"}, args...)
	}
	log.Printf("%s ...", strings.Join(args, " "))
	args = append(args, command...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type DefaultEnvironment struct{}

func (env *DefaultEnvironment) Execute(command []string) error {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
